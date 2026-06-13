package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

func TestEngineFallbackOnProviderFaults(t *testing.T) {
	tests := []struct {
		name         string
		behavior     string
		timeout      time.Duration
		wantFallback bool
	}{
		{name: "invalid json", behavior: "invalid_json", wantFallback: true},
		{name: "empty response", behavior: "empty", wantFallback: true},
		{name: "provider error", behavior: "error", wantFallback: true},
		{name: "illegal only", behavior: "illegal", wantFallback: true},
		{name: "slow timeout", behavior: "slow", timeout: time.Millisecond, wantFallback: true},
		{name: "mixed illegal and legal", behavior: "mixed_illegal", wantFallback: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Mode:     strategy.ModePure,
				Provider: providers.MockProvider{Behavior: tt.behavior},
			}
			if tt.timeout > 0 {
				opts.MoveTimeout = tt.timeout
			}
			e := New(opts)
			dec, state, err := e.ChooseMove(context.Background())
			if err != nil {
				t.Fatalf("choose: %v", err)
			}
			if dec.FallbackUsed != tt.wantFallback {
				t.Fatalf("fallback = %t, want %t", dec.FallbackUsed, tt.wantFallback)
			}
			if state.Snapshot.Ply != 1 {
				t.Fatalf("ply = %d, want 1", state.Snapshot.Ply)
			}
			if state.SchemaVersion != GameStateSchemaVersion {
				t.Fatalf("state schema_version = %q, want %q", state.SchemaVersion, GameStateSchemaVersion)
			}
			if state.Snapshot.MoveHistory[0].UCI == "" {
				t.Fatalf("selected invalid UCI: %+v", state.Snapshot.MoveHistory[0])
			}
		})
	}
}

func TestEngineRejectsIllegalUserMove(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e5"); err == nil {
		t.Fatal("expected illegal move error")
	}
}

func TestEngineUndoClearsFutureHistory(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e4"); err != nil {
		t.Fatalf("move 1: %v", err)
	}
	if _, err := e.ApplyUserMove(context.Background(), "e7e5"); err != nil {
		t.Fatalf("move 2: %v", err)
	}
	state, err := e.Undo(context.Background(), 1)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(state.Snapshot.MoveHistory) != 1 {
		t.Fatalf("history length = %d, want 1", len(state.Snapshot.MoveHistory))
	}
}

func TestEngineClockStateAndTimeoutOutcome(t *testing.T) {
	e := New(Options{})
	state, err := e.NewGame(context.Background(), NewGameOptions{
		Side:        "white",
		TimeControl: TimeControl{InitialMS: 300000, IncrementMS: 2000},
	})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	if !state.Clock.Enabled || state.Clock.WhiteMS != 300000 || state.Clock.BlackMS != 300000 || state.Clock.IncrementMS != 2000 {
		t.Fatalf("unexpected clock: %+v", state.Clock)
	}
	state, err = e.ApplyUserMove(context.Background(), "e2e4")
	if err != nil {
		t.Fatalf("user move: %v", err)
	}
	if state.Clock.WhiteMS != 302000 {
		t.Fatalf("white clock after increment = %d, want 302000", state.Clock.WhiteMS)
	}

	e.clock.WhiteMS = 0
	state, err = e.State(context.Background())
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Snapshot.Outcome.Status != "timeout" || state.Snapshot.Outcome.Winner != "black" {
		t.Fatalf("timeout outcome not represented: %+v", state.Snapshot.Outcome)
	}
	if len(state.Snapshot.LegalMoves) != 0 {
		t.Fatalf("timeout state kept legal moves: %d", len(state.Snapshot.LegalMoves))
	}
	if _, err := e.ApplyUserMove(context.Background(), "e7e5"); err == nil {
		t.Fatal("expected move after timeout to fail")
	}
}

func TestEngineNewGamePersistsVariantMetadata(t *testing.T) {
	e := New(Options{})
	state, err := e.NewGame(context.Background(), NewGameOptions{
		Side:    "white",
		Variant: "chess960",
		Seed:    42,
	})
	if err != nil {
		t.Fatalf("new Chess960 game: %v", err)
	}
	if state.Variant.Variant != "chess960" || state.Variant.FEN != state.InitialFEN || !state.Variant.CastlingEnabled || state.Variant.CastlingMode != chesscore.CastlingModeChess960External {
		t.Fatalf("unexpected Chess960 metadata: %+v initial=%s", state.Variant, state.InitialFEN)
	}
	if state.Snapshot.FEN == "" || len(state.Snapshot.LegalMoves) == 0 {
		t.Fatalf("Chess960 state not playable: %+v", state.Snapshot)
	}

	restored := New(Options{})
	restoredState, err := restored.LoadState(context.Background(), *state)
	if err != nil {
		t.Fatalf("load Chess960 state: %v", err)
	}
	if restoredState.Variant.Variant != "chess960" || restoredState.InitialFEN != state.InitialFEN {
		t.Fatalf("restored variant mismatch: got %+v initial=%s want %+v initial=%s", restoredState.Variant, restoredState.InitialFEN, state.Variant, state.InitialFEN)
	}
}

func TestEngineNewGameSupportsCustomBoardDefinition(t *testing.T) {
	def := chesscore.CustomBoardDefinition{
		SchemaVersion: chesscore.CustomBoardDefinitionSchemaVersion,
		ID:            "archbishop-lab",
		Name:          "Archbishop Lab",
		InitialFEN:    "4k3/8/8/8/3A4/8/8/4K3 w - - 0 1",
		RuleSet:       "custom-piece-lab",
		BoardWidth:    8,
		BoardHeight:   8,
		PieceRules: []chesscore.CustomPieceRule{{
			Symbol: "A",
			Name:   "Archbishop",
			Move:   "bishop+knight",
		}},
	}
	e := New(Options{})
	state, err := e.NewGame(context.Background(), NewGameOptions{
		Side:            "white",
		Variant:         chesscore.VariantCustom,
		BoardDefinition: &def,
	})
	if err != nil {
		t.Fatalf("new custom game: %v", err)
	}
	if state.Variant.BoardDefinition == nil || state.Features.LegalMoveCount == 0 || !containsLegalMove(state.Snapshot.LegalMoves, "d4e6") {
		t.Fatalf("custom state not playable: variant=%+v features=%+v moves=%+v", state.Variant, state.Features, state.Snapshot.LegalMoves)
	}
	state, err = e.ApplyUserMove(context.Background(), "d4e6")
	if err != nil {
		t.Fatalf("custom user move: %v", err)
	}
	if state.Snapshot.Board["e6"] != "A" || !strings.Contains(state.Snapshot.PGN, "1. Ae6") {
		t.Fatalf("custom move not reflected in state: board=%+v pgn=%s", state.Snapshot.Board, state.Snapshot.PGN)
	}
	restored := New(Options{})
	restoredState, err := restored.LoadState(context.Background(), *state)
	if err != nil {
		t.Fatalf("load custom state: %v", err)
	}
	if restoredState.Snapshot.Board["e6"] != "A" || restoredState.Variant.BoardDefinition == nil {
		t.Fatalf("custom state did not restore: %+v", restoredState)
	}
}

func TestEngineExportPGNIncludesNoema64MetadataAndComments(t *testing.T) {
	e := New(Options{
		Mode:     strategy.ModeHybrid,
		Provider: providers.MockProvider{},
		Verifier: verifier.StaticVerifier{},
	})
	if _, _, err := e.ChooseMove(context.Background()); err != nil {
		t.Fatalf("choose move: %v", err)
	}
	pgn, err := e.ExportPGN(context.Background())
	if err != nil {
		t.Fatalf("export pgn: %v", err)
	}
	for _, want := range []string{
		`[Annotator "Noema64"]`,
		`[EngineMode "hybrid"]`,
		`[LLMProvider "mock"]`,
		`[PromptVersion "move_selection/1.0.0"]`,
		`[Verifier "static_safety"]`,
		"{Plan:",
	} {
		if !strings.Contains(pgn, want) {
			t.Fatalf("PGN missing %q:\n%s", want, pgn)
		}
	}
}

func TestAnalyzePositionDoesNotMutateGameState(t *testing.T) {
	e := New(Options{Provider: providers.MockProvider{}})
	before, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("state before: %v", err)
	}
	dec, err := e.AnalyzePosition(context.Background())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !dec.AnalysisOnly {
		t.Fatalf("analysis decision was not marked analysis-only: %+v", dec)
	}
	after, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("state after: %v", err)
	}
	if after.Snapshot.Ply != before.Snapshot.Ply || after.Snapshot.FEN != before.Snapshot.FEN {
		t.Fatalf("analysis mutated game state: before=%+v after=%+v", before.Snapshot, after.Snapshot)
	}
	if after.LastDecision != nil {
		t.Fatalf("analysis replaced last played decision: %+v", after.LastDecision)
	}
	if after.StrategyMemory.LastUpdate.DecisionID != before.StrategyMemory.LastUpdate.DecisionID {
		t.Fatalf("analysis mutated strategy memory: before=%+v after=%+v", before.StrategyMemory.LastUpdate, after.StrategyMemory.LastUpdate)
	}
}

func TestEngineStateIncludesFeaturesAndStrategyMetrics(t *testing.T) {
	e := New(Options{Provider: providers.MockProvider{}})
	initial, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("initial state: %v", err)
	}
	if initial.Features.SideToMove != "white" || initial.Features.LegalMoveCount == 0 {
		t.Fatalf("features not populated on initial state: %+v", initial.Features)
	}
	if initial.StrategyMetrics.SchemaVersion != strategy.MemoryMetricsSchemaVersion || initial.StrategyMetrics.Quality <= 0 {
		t.Fatalf("strategy metrics not populated on initial state: %+v", initial.StrategyMetrics)
	}

	state, err := e.ApplyUserMove(context.Background(), "e2e4")
	if err != nil {
		t.Fatalf("user move: %v", err)
	}
	if state.Features.SideToMove != "black" || state.Features.Phase == "" {
		t.Fatalf("features not updated after move: %+v", state.Features)
	}
}

func TestEngineResignStopsGameAndRestoresFromState(t *testing.T) {
	e := New(Options{})
	state, err := e.Resign(context.Background(), "white")
	if err != nil {
		t.Fatalf("resign: %v", err)
	}
	if state.Snapshot.Outcome.Status != "resignation" || state.Snapshot.Outcome.Winner != "black" {
		t.Fatalf("unexpected resignation outcome: %+v", state.Snapshot.Outcome)
	}
	if len(state.Snapshot.LegalMoves) != 0 {
		t.Fatalf("resigned state kept legal moves: %d", len(state.Snapshot.LegalMoves))
	}
	if _, err := e.ApplyUserMove(context.Background(), "e2e4"); err == nil {
		t.Fatal("expected move after resignation to fail")
	}

	restored := New(Options{})
	restoredState, err := restored.LoadState(context.Background(), *state)
	if err != nil {
		t.Fatalf("load resigned state: %v", err)
	}
	if restoredState.Snapshot.Outcome.Status != "resignation" || restoredState.Snapshot.Outcome.Winner != "black" {
		t.Fatalf("restored outcome = %+v, want black resignation win", restoredState.Snapshot.Outcome)
	}
	if len(restoredState.Snapshot.LegalMoves) != 0 {
		t.Fatalf("restored resigned state kept legal moves: %d", len(restoredState.Snapshot.LegalMoves))
	}
}

func TestEngineResignCancelsLateEngineMove(t *testing.T) {
	provider := &lateProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	e := New(Options{Provider: provider})
	type chooseResult struct {
		state *GameState
		err   error
	}
	done := make(chan chooseResult, 1)
	go func() {
		_, state, err := e.ChooseMove(context.Background())
		done <- chooseResult{state: state, err: err}
	}()

	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	resigned, err := e.Resign(context.Background(), "white")
	if err != nil {
		t.Fatalf("resign: %v", err)
	}
	if resigned.Snapshot.Outcome.Status != "resignation" {
		t.Fatalf("resigned outcome = %+v", resigned.Snapshot.Outcome)
	}
	close(provider.release)

	select {
	case result := <-done:
		if result.err == nil || !strings.Contains(result.err.Error(), "cancelled") {
			t.Fatalf("late search err = %v, want cancellation", result.err)
		}
		if result.state == nil || result.state.Snapshot.Outcome.Status != "resignation" {
			t.Fatalf("late search state = %+v", result.state)
		}
	case <-time.After(time.Second):
		t.Fatal("engine search did not finish after release")
	}

	final, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("final state: %v", err)
	}
	if final.Snapshot.Ply != 0 || final.Snapshot.Outcome.Status != "resignation" {
		t.Fatalf("late engine move mutated resigned game: %+v", final.Snapshot)
	}
}

func TestEngineLoadStateRestoresMovesAndStrategyMemory(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e4"); err != nil {
		t.Fatalf("user move: %v", err)
	}
	if _, _, err := e.ChooseMove(context.Background()); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	saved, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("state: %v", err)
	}

	restored := New(Options{})
	state, err := restored.LoadState(context.Background(), *saved)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Snapshot.GameID != saved.Snapshot.GameID {
		t.Fatalf("game id = %s, want %s", state.Snapshot.GameID, saved.Snapshot.GameID)
	}
	if saved.SchemaVersion != GameStateSchemaVersion || state.SchemaVersion != GameStateSchemaVersion {
		t.Fatalf("schema versions saved=%q restored=%q, want %q", saved.SchemaVersion, state.SchemaVersion, GameStateSchemaVersion)
	}
	if len(state.Snapshot.MoveHistory) != len(saved.Snapshot.MoveHistory) {
		t.Fatalf("history length = %d, want %d", len(state.Snapshot.MoveHistory), len(saved.Snapshot.MoveHistory))
	}
	if state.StrategyMemory.LastUpdate.MovePlayed == "" {
		t.Fatalf("strategy memory did not restore last update: %+v", state.StrategyMemory)
	}
	if len(state.Snapshot.LegalMoves) == 0 && state.Snapshot.Outcome.Status == "ongoing" {
		t.Fatalf("ongoing restored game has no legal moves")
	}
}

func TestEngineLoadStateRejectsUnsupportedGameStateSchema(t *testing.T) {
	e := New(Options{})
	state, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	state.SchemaVersion = "game-state.v99"
	if _, err := e.LoadState(context.Background(), *state); err == nil {
		t.Fatal("expected unsupported game state schema to fail")
	}
}

type lateProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p *lateProvider) Name() string {
	return "late"
}

func (p *lateProvider) Capabilities() providers.Capabilities {
	return providers.MockProvider{}.Capabilities()
}

func (p *lateProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *lateProvider) CompleteJSON(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	close(p.started)
	<-p.release
	return providers.MockProvider{}.CompleteJSON(context.Background(), req)
}

func containsLegalMove(moves []chesscore.LegalMove, uci string) bool {
	for _, move := range moves {
		if move.UCI == uci {
			return true
		}
	}
	return false
}
