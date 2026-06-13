package appsvc

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/experiments"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

type Application struct {
	settingsPath    string
	settings        storage.Settings
	engine          *engine.Engine
	traces          *storage.TraceStore
	games           *storage.GameStore
	eventSink       EventSink
	settingsLoadErr error
}

const (
	maxFENImportBytes = 512
	maxPGNImportBytes = 1 << 20
)

type EventSink func(name string, payload any)

type MoveComparison struct {
	RequestedMove string `json:"requested_move"`
	SelectedMove  string `json:"selected_move"`
	Summary       string `json:"summary"`
	Requested     any    `json:"requested,omitempty"`
	Selected      any    `json:"selected,omitempty"`
}

func NewApplication(settingsPath string) *Application {
	settings, err := storage.LoadSettings(settingsPath)
	var settingsLoadErr error
	if err != nil {
		if errors.Is(err, storage.ErrUnsupportedSchema) {
			settingsLoadErr = appErr("ERR_SETTINGS_UNSUPPORTED_SCHEMA", err, true)
		}
		settings = storage.DefaultSettings()
	}
	app := &Application{
		settingsPath:    settingsPath,
		settings:        settings,
		traces:          storage.NewTraceStore(settings.Logging.OutputDir),
		games:           storage.NewGameStore(gameStoreDir(settings)),
		settingsLoadErr: settingsLoadErr,
	}
	app.engine = engine.New(app.engineOptions())
	app.restoreLatestGame()
	return app
}

func (a *Application) engineOptions() engine.Options {
	var provider providers.Provider = providers.MockProvider{}
	if a.settings.LLM.Provider == "openai_compatible" && a.settings.LLM.Endpoint != "" {
		provider = providers.OpenAICompatible{BaseURL: a.settings.LLM.Endpoint, APIKey: a.settings.LLM.APIKey, Model: a.settings.LLM.Model, Retries: a.settings.LLM.Retries}
	}
	var verify verifier.Verifier = verifier.StaticVerifier{Enabled: a.settings.Verifier.Enabled}
	if a.settings.Verifier.Enabled && a.settings.Verifier.Path != "" {
		verify = verifier.ExternalUCI{
			Path:             a.settings.Verifier.Path,
			MoveTimeMS:       a.settings.Verifier.MoveTimeMS,
			MaxCentipawnLoss: a.settings.Verifier.MaxCentipawnLoss,
		}
	}
	if a.settings.Verifier.TablebaseEnabled && a.settings.Verifier.TablebasePath != "" {
		verify = verifier.TablebaseVerifier{
			Base:    verify,
			Probe:   verifier.ExternalTablebase{Path: a.settings.Verifier.TablebasePath, TimeoutMS: a.settings.Verifier.TablebaseTimeoutMS},
			Enabled: true,
		}
	}
	timeout := time.Duration(a.settings.LLM.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	return engine.Options{
		Mode:            a.settings.Engine.DefaultMode,
		Personality:     a.settings.Engine.Personality,
		Provider:        provider,
		Verifier:        verify,
		Model:           a.settings.LLM.Model,
		Temperature:     a.settings.LLM.Temperature,
		MaxTokens:       a.settings.LLM.MaxTokens,
		ProviderRetries: a.settings.LLM.Retries,
		MaxCandidates:   a.settings.Engine.MaxCandidates,
		MoveTimeout:     timeout,
		LogRawPrompts:   a.settings.Privacy.LogRawPrompts,
		LogRawResponse:  a.settings.Privacy.LogRawLLMResponses,
		Progress:        a.emitDecisionProgress,
	}
}

func (a *Application) SetEventSink(sink EventSink) {
	a.eventSink = sink
	a.engine.SetOptions(a.engineOptions())
}

func (a *Application) emitDecisionProgress(event decision.ProgressEvent) {
	if a.eventSink == nil {
		return
	}
	a.eventSink(decision.DecisionStageEvent, event)
}

func (a *Application) NewGame(opts engine.NewGameOptions) (*engine.GameState, error) {
	if opts.Mode == "" {
		opts.Mode = a.settings.Engine.DefaultMode
	}
	if opts.Personality == "" {
		opts.Personality = a.settings.Engine.Personality
	}
	state, err := a.engine.NewGame(context.Background(), opts)
	if err != nil {
		return state, appErr("ERR_NEW_GAME", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) GetGame() (*engine.GameState, error) {
	state, err := a.engine.State(context.Background())
	return state, appErr("ERR_GAME_STATE", err, true)
}

func (a *Application) LegalMoves() (any, error) {
	moves, err := a.engine.LegalMoves(context.Background())
	return moves, appErr("ERR_LEGAL_MOVES", err, true)
}

func (a *Application) MakeUserMove(moveUCI string) (*engine.GameState, error) {
	state, err := a.engine.ApplyUserMove(context.Background(), moveUCI)
	if err != nil {
		return state, appErr("ERR_INVALID_MOVE", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) RequestEngineMove() (any, error) {
	dec, state, err := a.engine.ChooseMove(context.Background())
	if err == nil && a.settings.Engine.TraceEnabled {
		traceStageStarted := time.Now()
		a.emitDecisionProgress(decision.ProgressEvent{
			EventName:  decision.DecisionStageEvent,
			DecisionID: dec.DecisionID,
			GameID:     dec.GameID,
			Stage:      "writing_trace",
			Status:     "started",
			Message:    "Persist decision trace.",
			Timestamp:  traceStageStarted.UTC().Format(time.RFC3339Nano),
		})
		dec.Stages = append(dec.Stages, decision.CompletedStage("writing_trace", "completed", "Decision trace persisted.", traceStageStarted, time.Now()))
		_ = a.traces.AppendDecision(context.Background(), dec)
		a.emitDecisionProgress(decision.ProgressEvent{
			EventName:  decision.DecisionStageEvent,
			DecisionID: dec.DecisionID,
			GameID:     dec.GameID,
			Stage:      "writing_trace",
			Status:     "completed",
			Message:    "Decision trace persisted.",
			ElapsedMS:  time.Since(traceStageStarted).Milliseconds(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	if err != nil {
		return map[string]any{"decision": dec, "state": state}, appErr("ERR_ENGINE_MOVE", err, true)
	}
	return map[string]any{"decision": dec, "state": state}, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) AnalyzeCurrentPosition() (*decision.MoveDecision, error) {
	dec, err := a.engine.AnalyzePosition(context.Background())
	if err == nil && a.settings.Engine.TraceEnabled {
		traceStageStarted := time.Now()
		a.emitDecisionProgress(decision.ProgressEvent{
			EventName:  decision.DecisionStageEvent,
			DecisionID: dec.DecisionID,
			GameID:     dec.GameID,
			Stage:      "writing_trace",
			Status:     "started",
			Message:    "Persist analysis trace.",
			Timestamp:  traceStageStarted.UTC().Format(time.RFC3339Nano),
		})
		dec.Stages = append(dec.Stages, decision.CompletedStage("writing_trace", "completed", "Analysis trace persisted.", traceStageStarted, time.Now()))
		_ = a.traces.AppendDecision(context.Background(), dec)
		a.emitDecisionProgress(decision.ProgressEvent{
			EventName:  decision.DecisionStageEvent,
			DecisionID: dec.DecisionID,
			GameID:     dec.GameID,
			Stage:      "writing_trace",
			Status:     "completed",
			Message:    "Analysis trace persisted.",
			ElapsedMS:  time.Since(traceStageStarted).Milliseconds(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return dec, appErr("ERR_ANALYSIS", err, true)
}

func (a *Application) StopEngine() error {
	return appErr("ERR_CANCELLED", a.engine.Stop(context.Background()), true)
}

func (a *Application) Resign(side string) (*engine.GameState, error) {
	state, err := a.engine.Resign(context.Background(), side)
	if err != nil {
		return state, appErr("ERR_RESIGN", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) Undo(plies int) (*engine.GameState, error) {
	state, err := a.engine.Undo(context.Background(), plies)
	if err != nil {
		return state, appErr("ERR_UNDO", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) ExportPGN() (string, error) {
	pgn, err := a.engine.ExportPGN(context.Background())
	return pgn, appErr("ERR_EXPORT_PGN", err, true)
}

func (a *Application) ExportFEN() (string, error) {
	fen, err := a.engine.ExportFEN(context.Background())
	return fen, appErr("ERR_EXPORT_FEN", err, true)
}

func (a *Application) ExportTrace() (string, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return "", appErr("ERR_GAME_STATE", err, true)
	}
	trace, err := a.traces.ReadGame(context.Background(), state.Snapshot.GameID)
	return trace, appErr("ERR_EXPORT_TRACE", err, true)
}

func (a *Application) WhyNotMove(moveUCI string) (*MoveComparison, error) {
	moveUCI = strings.TrimSpace(moveUCI)
	if moveUCI == "" {
		return nil, &AppError{Code: "ERR_INVALID_MOVE", Message: "Move is required", Recoverable: true}
	}
	state, err := a.engine.State(context.Background())
	if err != nil {
		return nil, appErr("ERR_GAME_STATE", err, true)
	}
	dec := state.LastDecision
	if dec == nil {
		return nil, &AppError{Code: "ERR_NO_DECISION", Message: "No engine decision is available to compare against.", Recoverable: true}
	}
	selected := dec.SelectedMove.UCI
	var requestedCandidate any
	var selectedCandidate any
	for _, candidate := range dec.CandidateMoves {
		if candidate.UCI == moveUCI || strings.EqualFold(candidate.SAN, moveUCI) {
			requestedCandidate = candidate
			moveUCI = candidate.UCI
		}
		if candidate.UCI == selected {
			selectedCandidate = candidate
		}
	}
	summary := "Noema64 selected " + selected + " because: " + dec.Explanation
	if moveUCI == selected {
		summary = "That is the selected engine move. " + dec.Explanation
	} else if requestedCandidate != nil {
		summary = "Noema64 preferred " + selected + " over " + moveUCI + " based on final score, verifier status, and plan alignment in the recorded decision trace."
	} else {
		summary = "Move " + moveUCI + " was not in the recorded candidate set for the last engine decision. Compare it from the analysis view before applying moves."
	}
	return &MoveComparison{
		RequestedMove: moveUCI,
		SelectedMove:  selected,
		Summary:       summary,
		Requested:     requestedCandidate,
		Selected:      selectedCandidate,
	}, nil
}

func (a *Application) ImportFEN(fen string) (*engine.GameState, error) {
	fen = strings.TrimSpace(fen)
	if fen == "" {
		return nil, &AppError{Code: "ERR_IMPORT_INVALID_FEN", Message: "FEN is required", Recoverable: true}
	}
	if len(fen) > maxFENImportBytes {
		return nil, &AppError{Code: "ERR_IMPORT_TOO_LARGE", Message: "FEN import is too large", Recoverable: true}
	}
	state, err := a.engine.NewGame(context.Background(), engine.NewGameOptions{
		Side:        "auto",
		FEN:         fen,
		Mode:        a.settings.Engine.DefaultMode,
		Personality: a.settings.Engine.Personality,
	})
	if err != nil {
		return state, appErr("ERR_IMPORT_INVALID_FEN", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) ImportPGN(pgn string) (*engine.GameState, error) {
	pgn = strings.TrimSpace(pgn)
	if pgn == "" {
		return nil, &AppError{Code: "ERR_IMPORT_INVALID_PGN", Message: "PGN is required", Recoverable: true}
	}
	if len(pgn) > maxPGNImportBytes {
		return nil, &AppError{Code: "ERR_IMPORT_TOO_LARGE", Message: "PGN import is too large", Recoverable: true}
	}
	state, err := a.engine.LoadPGN(context.Background(), pgn)
	if err != nil {
		return state, appErr("ERR_IMPORT_INVALID_PGN", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) GetSettings() (storage.Settings, error) {
	settings := a.settings
	if settings.LLM.APIKey != "" {
		settings.LLM.APIKey = "[REDACTED]"
	}
	settings.LLM.Profiles = append([]storage.ProviderProfile(nil), settings.LLM.Profiles...)
	for i := range settings.LLM.Profiles {
		if settings.LLM.Profiles[i].APIKey != "" {
			settings.LLM.Profiles[i].APIKey = "[REDACTED]"
		}
	}
	if a.settingsLoadErr != nil {
		return settings, a.settingsLoadErr
	}
	return settings, nil
}

func (a *Application) SaveSettings(settings storage.Settings) error {
	if settings.LLM.APIKey == "[REDACTED]" {
		settings.LLM.APIKey = a.settings.LLM.APIKey
	}
	preserveRedactedProfileKeys(&settings, a.settings)
	settings = storage.NormalizeSettings(settings)
	if settings.LLM.Provider == "openai_compatible" && !settings.Privacy.CloudProviderWarningAcknowledged {
		return &AppError{Code: "ERR_PRIVACY_ACK_REQUIRED", Message: "Cloud provider data sharing must be acknowledged before saving this provider.", Recoverable: true}
	}
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return appErr("ERR_SETTINGS_INVALID", err, true)
	}
	settings = storage.NormalizeSettings(settings)
	a.settings = settings
	a.settingsLoadErr = nil
	a.engine.SetOptions(a.engineOptions())
	a.traces = storage.NewTraceStore(settings.Logging.OutputDir)
	a.games = storage.NewGameStore(gameStoreDir(settings))
	state, err := a.engine.State(context.Background())
	if err != nil {
		return appErr("ERR_GAME_STATE", err, true)
	}
	return appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func preserveRedactedProfileKeys(settings *storage.Settings, previous storage.Settings) {
	previousKeys := map[string]string{}
	for _, profile := range previous.LLM.Profiles {
		if profile.APIKey != "" {
			previousKeys[profile.ID] = profile.APIKey
		}
	}
	for i := range settings.LLM.Profiles {
		if settings.LLM.Profiles[i].APIKey == "[REDACTED]" {
			settings.LLM.Profiles[i].APIKey = previousKeys[settings.LLM.Profiles[i].ID]
		}
	}
}

func (a *Application) RecentGames(limit int) ([]storage.GameRecord, error) {
	records, err := a.games.List(context.Background(), limit)
	return records, appErr("ERR_RECENT_GAMES", err, true)
}

func (a *Application) LoadRecentGame(gameID string) (*engine.GameState, error) {
	record, err := a.games.Load(context.Background(), gameID)
	if err != nil {
		return nil, appErr("ERR_RECENT_GAMES", err, true)
	}
	state, err := a.engine.LoadState(context.Background(), record.State)
	if err != nil {
		return nil, appErr("ERR_RECENT_GAMES", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func (a *Application) HealthCheckProvider() (map[string]any, error) {
	provider := a.engineOptions().Provider
	healthCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := provider.HealthCheck(healthCtx)
	return map[string]any{
		"provider":     provider.Name(),
		"healthy":      err == nil,
		"capabilities": provider.Capabilities(),
	}, appErr("ERR_PROVIDER_HEALTH", err, true)
}

func (a *Application) RunRandomBenchmark(games int, seed int64) (experiments.Summary, error) {
	if games <= 0 {
		games = 100
	}
	opts := a.engineOptions()
	opts.Mode = strategy.ModePure
	opts.Verifier = verifier.LegalOnlyVerifier{}
	runner := experiments.Runner{Options: opts}
	summary, err := runner.RandomLegalBenchmark(context.Background(), games, seed)
	return summary, appErr("ERR_EXPERIMENT", err, true)
}

func (a *Application) RunModeBenchmark(games int, seed int64) (experiments.ModeBenchmarkSummary, error) {
	if games <= 0 {
		games = 20
	}
	opts := a.engineOptions()
	opts.Verifier = verifier.StaticVerifier{}
	runner := experiments.Runner{Options: opts}
	summary, err := runner.RandomLegalModeBenchmark(context.Background(), games, seed, []strategy.EngineMode{
		strategy.ModePure,
		strategy.ModeBlunderguard,
		strategy.ModeHybrid,
	})
	return summary, appErr("ERR_EXPERIMENT", err, true)
}

func (a *Application) Modes() []strategy.EngineMode {
	return []strategy.EngineMode{strategy.ModePure, strategy.ModeBlunderguard, strategy.ModeHybrid, strategy.ModeCoach}
}

func (a *Application) restoreLatestGame() {
	record, err := a.games.LoadLatest(context.Background())
	if err != nil {
		return
	}
	_, _ = a.engine.LoadState(context.Background(), record.State)
}

func (a *Application) persistGameState(state *engine.GameState) error {
	if a.games == nil {
		return nil
	}
	return a.games.Save(context.Background(), state)
}

func gameStoreDir(settings storage.Settings) string {
	outputDir := strings.TrimSpace(settings.Logging.OutputDir)
	if outputDir == "" {
		outputDir = "logs"
	}
	return filepath.Join(outputDir, "games")
}
