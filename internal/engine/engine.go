package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

type Options struct {
	Mode            strategy.EngineMode
	Personality     strategy.Personality
	Provider        providers.Provider
	Verifier        verifier.Verifier
	Model           string
	Temperature     float64
	MaxTokens       int
	ProviderRetries int
	MaxCandidates   int
	MoveTimeout     time.Duration
	LogRawPrompts   bool
	LogRawResponse  bool
	Progress        decision.ProgressFunc
}

type NewGameOptions struct {
	Side        string               `json:"side"`
	FEN         string               `json:"fen,omitempty"`
	Variant     chesscore.Variant    `json:"variant,omitempty"`
	Seed        int64                `json:"seed,omitempty"`
	TimeControl TimeControl          `json:"time_control,omitempty"`
	Mode        strategy.EngineMode  `json:"mode,omitempty"`
	Personality strategy.Personality `json:"personality,omitempty"`
}

type TimeControl struct {
	InitialMS   int64 `json:"initial_ms"`
	IncrementMS int64 `json:"increment_ms"`
}

const GameStateSchemaVersion = "game-state.v1"

type ClockState struct {
	Enabled     bool  `json:"enabled"`
	WhiteMS     int64 `json:"white_ms"`
	BlackMS     int64 `json:"black_ms"`
	IncrementMS int64 `json:"increment_ms"`
}

type GameState struct {
	SchemaVersion   string                   `json:"schema_version"`
	Snapshot        chesscore.Snapshot       `json:"snapshot"`
	Features        chesscore.FeatureSummary `json:"features"`
	InitialFEN      string                   `json:"initial_fen"`
	Variant         chesscore.VariantStart   `json:"variant"`
	AppliedMoves    []string                 `json:"applied_moves"`
	Clock           ClockState               `json:"clock"`
	StrategyMemory  strategy.StrategyMemory  `json:"strategy_memory"`
	StrategyMetrics strategy.MemoryMetrics   `json:"strategy_metrics"`
	LastDecision    *decision.MoveDecision   `json:"last_decision,omitempty"`
}

type Engine struct {
	mu           sync.Mutex
	game         *chesscore.Game
	variant      chesscore.VariantStart
	memory       strategy.StrategyMemory
	lastDecision *decision.MoveDecision
	clock        ClockState
	opts         Options
	cancel       context.CancelFunc
	activeID     string
}

func New(opts Options) *Engine {
	opts = normalizeOptions(opts)
	game := chesscore.NewGame()
	return &Engine{
		game:    game,
		variant: chesscore.StandardStart(game.InitialFEN()),
		memory:  strategy.NewMemory(game.ID(), "white"),
		opts:    opts,
	}
}

func (e *Engine) NewGame(ctx context.Context, opts NewGameOptions) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	game, variant, err := newVariantGame(opts)
	if err != nil {
		return nil, err
	}
	e.game = game
	e.variant = variant
	e.clock = newClock(opts.TimeControl)
	if opts.Mode != "" {
		e.opts.Mode = opts.Mode
	}
	if opts.Personality != "" {
		e.opts.Personality = opts.Personality
	}
	side := opts.Side
	if side == "" || side == "auto" {
		side = game.SideToMove()
	}
	e.memory = strategy.NewMemory(game.ID(), side)
	e.lastDecision = nil
	return e.stateLocked(), nil
}

func (e *Engine) ApplyUserMove(ctx context.Context, moveUCI string) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.currentOutcomeLocked().Status != "ongoing" {
		return nil, fmt.Errorf("game is over")
	}
	movingSide := e.game.SideToMove()
	if _, err := e.game.ApplyUCI(moveUCI); err != nil {
		return nil, err
	}
	e.applyClockLocked(movingSide, 0)
	return e.stateLocked(), nil
}

func (e *Engine) LoadPGN(ctx context.Context, pgn string) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	game, err := chesscore.FromPGN(strings.NewReader(pgn))
	if err != nil {
		return nil, err
	}
	e.game = game
	e.variant = chesscore.StandardStart(game.InitialFEN())
	e.clock = ClockState{}
	e.memory = strategy.NewMemory(game.ID(), game.SideToMove())
	e.lastDecision = nil
	return e.stateLocked(), nil
}

func (e *Engine) LoadState(ctx context.Context, state GameState) (*GameState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	if version := strings.TrimSpace(state.SchemaVersion); version != "" && version != GameStateSchemaVersion {
		return nil, fmt.Errorf("game state schema_version %q is unsupported by this release", state.SchemaVersion)
	}
	game, err := gameFromState(ctx, state)
	if err != nil {
		return nil, err
	}
	e.game = game
	e.variant = chesscore.NormalizeVariantStart(state.Variant, game.InitialFEN())
	e.clock = state.Clock
	e.memory = state.StrategyMemory
	if strings.TrimSpace(e.memory.SchemaVersion) == "" {
		e.memory = strategy.NewMemory(game.ID(), game.SideToMove())
	}
	e.memory.GameID = game.ID()
	if strings.TrimSpace(e.memory.Side) == "" {
		e.memory.Side = game.SideToMove()
	}
	e.lastDecision = state.LastDecision
	return e.stateLocked(), nil
}

func (e *Engine) ChooseMove(ctx context.Context) (*decision.MoveDecision, *GameState, error) {
	e.mu.Lock()
	if e.currentOutcomeLocked().Status != "ongoing" {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("game is over")
	}
	if e.cancel != nil {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("engine is already thinking")
	}
	rootCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	activeID := fmt.Sprintf("%d", time.Now().UnixNano())
	e.activeID = activeID
	movingSide := e.game.SideToMove()
	start := time.Now()
	req := decision.Request{
		Game:            e.game.Clone(),
		Memory:          e.memory,
		Mode:            e.opts.Mode,
		Personality:     e.opts.Personality,
		Provider:        e.opts.Provider,
		Verifier:        e.opts.Verifier,
		Model:           e.opts.Model,
		Temperature:     e.opts.Temperature,
		MaxTokens:       e.opts.MaxTokens,
		ProviderRetries: e.opts.ProviderRetries,
		MaxCandidates:   e.opts.MaxCandidates,
		Timeout:         e.opts.MoveTimeout,
		LogRawPrompts:   e.opts.LogRawPrompts,
		LogRawResponse:  e.opts.LogRawResponse,
		Progress:        e.opts.Progress,
	}
	e.mu.Unlock()

	dec, err := decision.ChooseMove(rootCtx, req)

	e.mu.Lock()
	defer e.mu.Unlock()
	stillActive := e.activeID == activeID
	if stillActive {
		e.cancel = nil
		e.activeID = ""
	}
	if !stillActive {
		return nil, e.stateLocked(), fmt.Errorf("engine search was cancelled")
	}
	if e.currentOutcomeLocked().Status != "ongoing" {
		return nil, e.stateLocked(), fmt.Errorf("game is over")
	}
	if e.game.SideToMove() != movingSide {
		return nil, e.stateLocked(), fmt.Errorf("position changed while engine was thinking")
	}
	if err != nil {
		return nil, nil, err
	}
	if dec == nil {
		return nil, nil, fmt.Errorf("no move decision")
	}
	if !e.game.IsLegalUCI(dec.SelectedMove.UCI) {
		return nil, nil, fmt.Errorf("selector returned illegal move %s", dec.SelectedMove.UCI)
	}
	playStarted := time.Now()
	emitEngineProgress(e.opts.Progress, dec, "playing_move", "started", "Apply selected move to game state.", 0)
	if _, err := e.game.ApplyUCI(dec.SelectedMove.UCI); err != nil {
		emitEngineProgress(e.opts.Progress, dec, "playing_move", "failed", err.Error(), time.Since(playStarted).Milliseconds())
		return nil, nil, err
	}
	dec.Stages = append(dec.Stages, decision.CompletedStage("playing_move", "completed", "Applied selected move to game state.", playStarted, time.Now()))
	emitEngineProgress(e.opts.Progress, dec, "playing_move", "completed", "Applied selected move to game state.", time.Since(playStarted).Milliseconds())
	e.game.AnnotateLastMove("Plan: " + dec.Explanation)
	e.applyClockLocked(movingSide, time.Since(start).Milliseconds())
	e.memory = dec.StrategyAfter
	e.lastDecision = dec
	return dec, e.stateLocked(), nil
}

func (e *Engine) AnalyzePosition(ctx context.Context) (*decision.MoveDecision, error) {
	e.mu.Lock()
	if e.currentOutcomeLocked().Status != "ongoing" {
		e.mu.Unlock()
		return nil, fmt.Errorf("game is over")
	}
	if e.cancel != nil {
		e.mu.Unlock()
		return nil, fmt.Errorf("engine is already thinking")
	}
	rootCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	activeID := fmt.Sprintf("%d", time.Now().UnixNano())
	e.activeID = activeID
	req := decision.Request{
		Game:            e.game.Clone(),
		Memory:          e.memory,
		Mode:            e.opts.Mode,
		Personality:     e.opts.Personality,
		Provider:        e.opts.Provider,
		Verifier:        e.opts.Verifier,
		Model:           e.opts.Model,
		Temperature:     e.opts.Temperature,
		MaxTokens:       e.opts.MaxTokens,
		ProviderRetries: e.opts.ProviderRetries,
		MaxCandidates:   e.opts.MaxCandidates,
		Timeout:         e.opts.MoveTimeout,
		LogRawPrompts:   e.opts.LogRawPrompts,
		LogRawResponse:  e.opts.LogRawResponse,
		Progress:        e.opts.Progress,
	}
	e.mu.Unlock()

	dec, err := decision.ChooseMove(rootCtx, req)

	e.mu.Lock()
	defer e.mu.Unlock()
	stillActive := e.activeID == activeID
	if stillActive {
		e.cancel = nil
		e.activeID = ""
	}
	if !stillActive {
		return nil, fmt.Errorf("engine analysis was cancelled")
	}
	if err != nil {
		return nil, err
	}
	if dec == nil {
		return nil, fmt.Errorf("no move decision")
	}
	dec.AnalysisOnly = true
	return dec, nil
}

func (e *Engine) Resign(ctx context.Context, side string) (*GameState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	if strings.TrimSpace(side) == "" || strings.EqualFold(side, "auto") {
		side = e.game.SideToMove()
	}
	if e.currentOutcomeLocked().Status != "ongoing" {
		return nil, fmt.Errorf("game is over")
	}
	if err := e.game.Resign(side); err != nil {
		return nil, err
	}
	return e.stateLocked(), nil
}

func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	return nil
}

func (e *Engine) State(ctx context.Context) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stateLocked(), nil
}

func (e *Engine) SetOptions(opts Options) {
	e.mu.Lock()
	defer e.mu.Unlock()
	normalized := normalizeOptions(opts)
	if opts.Provider == nil {
		normalized.Provider = e.opts.Provider
	}
	if opts.Verifier == nil {
		normalized.Verifier = e.opts.Verifier
	}
	e.opts = normalized
}

func (e *Engine) LegalMoves(ctx context.Context) ([]chesscore.LegalMove, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.currentOutcomeLocked().Status != "ongoing" {
		return []chesscore.LegalMove{}, nil
	}
	return e.game.LegalMoves(), nil
}

func (e *Engine) ExportPGN(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return pgnWithMetadata(e.game.PGN(), e.opts, e.lastDecision, e.variant), nil
}

func (e *Engine) ExportFEN(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.game.FEN(), nil
}

func (e *Engine) Undo(ctx context.Context, plies int) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.game.Undo(plies)
	e.memory = strategy.NewMemory(e.game.ID(), e.game.SideToMove())
	e.lastDecision = nil
	return e.stateLocked(), nil
}

func (e *Engine) stateLocked() *GameState {
	snapshot := e.game.Snapshot()
	if outcome := e.currentOutcomeLocked(); outcome != snapshot.Outcome {
		snapshot.Outcome = outcome
		snapshot.LegalMoves = []chesscore.LegalMove{}
	}
	var previous *strategy.StrategyMemory
	if e.lastDecision != nil {
		previous = &e.lastDecision.StrategyBefore
	}
	return &GameState{
		SchemaVersion:   GameStateSchemaVersion,
		Snapshot:        snapshot,
		Features:        e.game.Features(),
		InitialFEN:      e.game.InitialFEN(),
		Variant:         chesscore.NormalizeVariantStart(e.variant, e.game.InitialFEN()),
		AppliedMoves:    e.game.AppliedUCI(),
		Clock:           e.clock,
		StrategyMemory:  e.memory,
		StrategyMetrics: strategy.EvaluateMemory(e.memory, previous),
		LastDecision:    e.lastDecision,
	}
}

func emitEngineProgress(progress decision.ProgressFunc, dec *decision.MoveDecision, stage, status, message string, elapsedMS int64) {
	if progress == nil || dec == nil {
		return
	}
	progress(decision.ProgressEvent{
		EventName:  decision.DecisionStageEvent,
		DecisionID: dec.DecisionID,
		GameID:     dec.GameID,
		Stage:      stage,
		Status:     status,
		Message:    message,
		ElapsedMS:  elapsedMS,
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
	})
}

type pgnTag struct {
	Name  string
	Value string
}

func pgnWithMetadata(pgn string, opts Options, dec *decision.MoveDecision, variant chesscore.VariantStart) string {
	body := strings.TrimSpace(pgn)
	if body == "" {
		body = "*"
	}
	added := []string{}
	for _, tag := range noemaPGNTags(opts, dec, variant) {
		if hasPGNTag(body, tag.Name) {
			continue
		}
		added = append(added, fmt.Sprintf("[%s \"%s\"]", tag.Name, pgnTagValue(tag.Value)))
	}
	if len(added) == 0 {
		return body
	}
	tagBlock := strings.Join(added, "\n")
	lines := strings.Split(body, "\n")
	tagEnd := 0
	for tagEnd < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[tagEnd]), "[") {
		tagEnd++
	}
	if tagEnd > 0 {
		head := strings.TrimRight(strings.Join(lines[:tagEnd], "\n"), "\n")
		tail := strings.TrimSpace(strings.Join(lines[tagEnd:], "\n"))
		if tail == "" {
			return head + "\n" + tagBlock
		}
		return head + "\n" + tagBlock + "\n\n" + tail
	}
	return tagBlock + "\n\n" + body
}

func noemaPGNTags(opts Options, dec *decision.MoveDecision, variant chesscore.VariantStart) []pgnTag {
	mode := opts.Mode
	if dec != nil && dec.Mode != "" {
		mode = dec.Mode
	}
	providerName := ""
	if dec != nil && dec.Provider.Name != "" {
		providerName = dec.Provider.Name
	} else if opts.Provider != nil {
		providerName = opts.Provider.Name()
	}
	verifierName := ""
	if dec != nil && dec.Assistance.VerifierName != "" {
		verifierName = dec.Assistance.VerifierName
	} else if dec != nil && dec.VerifierTrace != nil && dec.VerifierTrace.Name != "" {
		verifierName = dec.VerifierTrace.Name
	} else if opts.Verifier != nil {
		verifierName = opts.Verifier.Name()
	}
	variant = chesscore.NormalizeVariantStart(variant, "")
	tags := []pgnTag{
		{Name: "Annotator", Value: "Noema64"},
		{Name: "Variant", Value: string(variant.Variant)},
		{Name: "EngineMode", Value: string(mode)},
		{Name: "LLMProvider", Value: providerName},
		{Name: "PromptVersion", Value: strategy.PromptVersion},
		{Name: "Verifier", Value: verifierName},
	}
	if variant.Variant != chesscore.VariantStandard {
		tags = append(tags, pgnTag{Name: "InitialFEN", Value: variant.FEN})
	}
	return tags
}

func newVariantGame(opts NewGameOptions) (*chesscore.Game, chesscore.VariantStart, error) {
	fen := strings.TrimSpace(opts.FEN)
	variant := opts.Variant
	if variant == "" {
		if fen == "" {
			variant = chesscore.VariantStandard
		} else {
			variant = chesscore.VariantCustom
		}
	}
	switch variant {
	case chesscore.VariantStandard:
		if fen != "" {
			game, err := chesscore.FromFEN(fen)
			if err != nil {
				return nil, chesscore.VariantStart{}, err
			}
			return game, chesscore.StandardStart(game.InitialFEN()), nil
		}
		game := chesscore.NewGame()
		return game, chesscore.StandardStart(game.InitialFEN()), nil
	case chesscore.VariantChess960:
		start := chesscore.Chess960Start(opts.Seed)
		if fen != "" {
			custom, err := chesscore.CustomBoardStart(fen)
			if err != nil {
				return nil, chesscore.VariantStart{}, err
			}
			custom.Variant = chesscore.VariantChess960
			custom.Seed = opts.Seed
			custom.CastlingEnabled = false
			custom.Notes = append(custom.Notes, "Loaded as a Chess960-compatible custom start.")
			start = custom
		}
		game, err := chesscore.FromFEN(start.FEN)
		if err != nil {
			return nil, chesscore.VariantStart{}, err
		}
		return game, start, nil
	case chesscore.VariantCustom:
		start, err := chesscore.CustomBoardStart(fen)
		if err != nil {
			return nil, chesscore.VariantStart{}, err
		}
		game, err := chesscore.FromFEN(start.FEN)
		if err != nil {
			return nil, chesscore.VariantStart{}, err
		}
		return game, start, nil
	default:
		return nil, chesscore.VariantStart{}, fmt.Errorf("unsupported chess variant %q", opts.Variant)
	}
}

func hasPGNTag(pgn string, name string) bool {
	prefix := "[" + name + " "
	for _, line := range strings.Split(pgn, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "[") {
			return false
		}
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func pgnTagValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "unknown"
	}
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func newClock(tc TimeControl) ClockState {
	if tc.InitialMS <= 0 {
		return ClockState{}
	}
	increment := tc.IncrementMS
	if increment < 0 {
		increment = 0
	}
	return ClockState{
		Enabled:     true,
		WhiteMS:     tc.InitialMS,
		BlackMS:     tc.InitialMS,
		IncrementMS: increment,
	}
}

func (e *Engine) applyClockLocked(side string, elapsedMS int64) {
	if !e.clock.Enabled {
		return
	}
	if elapsedMS < 0 {
		elapsedMS = 0
	}
	switch side {
	case "white":
		e.clock.WhiteMS -= elapsedMS
		if e.clock.WhiteMS < 0 {
			e.clock.WhiteMS = 0
		}
		if e.clock.WhiteMS > 0 {
			e.clock.WhiteMS += e.clock.IncrementMS
		}
	case "black":
		e.clock.BlackMS -= elapsedMS
		if e.clock.BlackMS < 0 {
			e.clock.BlackMS = 0
		}
		if e.clock.BlackMS > 0 {
			e.clock.BlackMS += e.clock.IncrementMS
		}
	}
}

func (e *Engine) currentOutcomeLocked() chesscore.Outcome {
	if e.clock.Enabled {
		switch {
		case e.clock.WhiteMS <= 0 && e.clock.BlackMS <= 0:
			return chesscore.Outcome{Status: "draw", Method: "timeout"}
		case e.clock.WhiteMS <= 0:
			return chesscore.Outcome{Status: "timeout", Winner: "black", Method: "timeout"}
		case e.clock.BlackMS <= 0:
			return chesscore.Outcome{Status: "timeout", Winner: "white", Method: "timeout"}
		}
	}
	return e.game.Outcome()
}

func gameFromState(ctx context.Context, state GameState) (*chesscore.Game, error) {
	if state.Snapshot.Outcome.Status == "resignation" {
		return gameFromReplayState(ctx, state)
	}
	gameID := state.Snapshot.GameID
	pgn := strings.TrimSpace(state.Snapshot.PGN)
	if pgn != "" && pgn != "*" {
		if game, err := chesscore.FromPGNWithID(strings.NewReader(pgn), gameID); err == nil {
			return game, nil
		}
	}
	return gameFromReplayState(ctx, state)
}

func gameFromReplayState(ctx context.Context, state GameState) (*chesscore.Game, error) {
	gameID := state.Snapshot.GameID
	initialFEN := strings.TrimSpace(state.InitialFEN)
	if initialFEN == "" {
		initialFEN = strings.TrimSpace(state.Snapshot.FEN)
	}
	if initialFEN == "" {
		return nil, fmt.Errorf("saved game state is missing FEN")
	}
	game, err := chesscore.FromFENWithID(initialFEN, gameID)
	if err != nil {
		return nil, err
	}
	for _, moveUCI := range state.AppliedMoves {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if _, err := game.ApplyUCI(moveUCI); err != nil {
			return nil, fmt.Errorf("replay saved move %q: %w", moveUCI, err)
		}
	}
	if state.Snapshot.Outcome.Status == "resignation" {
		side, err := resigningSideFromOutcome(state.Snapshot.Outcome)
		if err != nil {
			return nil, err
		}
		if err := game.Resign(side); err != nil {
			return nil, err
		}
	}
	return game, nil
}

func resigningSideFromOutcome(outcome chesscore.Outcome) (string, error) {
	switch strings.ToLower(strings.TrimSpace(outcome.Winner)) {
	case "white":
		return "black", nil
	case "black":
		return "white", nil
	default:
		return "", fmt.Errorf("saved resignation outcome is missing a winner")
	}
}

func normalizeOptions(opts Options) Options {
	if opts.Mode == "" {
		opts.Mode = strategy.ModeBlunderguard
	}
	if opts.Personality == "" {
		opts.Personality = strategy.PersonalityBalanced
	}
	if opts.Provider == nil {
		opts.Provider = providers.MockProvider{}
	}
	if opts.Verifier == nil {
		opts.Verifier = verifier.StaticVerifier{}
	}
	if opts.Model == "" {
		opts.Model = "mock-balanced"
	}
	if opts.MaxCandidates == 0 {
		opts.MaxCandidates = 5
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 1600
	}
	if opts.MoveTimeout == 0 {
		opts.MoveTimeout = 12 * time.Second
	}
	return opts
}
