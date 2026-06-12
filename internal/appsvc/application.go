package appsvc

import (
	"context"
	"time"

	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/experiments"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

type Application struct {
	settingsPath string
	settings     storage.Settings
	engine       *engine.Engine
	traces       *storage.TraceStore
}

func NewApplication(settingsPath string) *Application {
	settings, err := storage.LoadSettings(settingsPath)
	if err != nil {
		settings = storage.DefaultSettings()
	}
	app := &Application{
		settingsPath: settingsPath,
		settings:     settings,
		traces:       storage.NewTraceStore(settings.Logging.OutputDir),
	}
	app.engine = engine.New(app.engineOptions())
	return app
}

func (a *Application) engineOptions() engine.Options {
	var provider providers.Provider = providers.MockProvider{}
	if a.settings.LLM.Provider == "openai_compatible" && a.settings.LLM.Endpoint != "" {
		provider = providers.OpenAICompatible{BaseURL: a.settings.LLM.Endpoint, APIKey: a.settings.LLM.APIKey}
	}
	var verify verifier.Verifier = verifier.StaticVerifier{Enabled: a.settings.Verifier.Enabled}
	if a.settings.Verifier.Enabled && a.settings.Verifier.Path != "" {
		verify = verifier.ExternalUCI{Path: a.settings.Verifier.Path, MoveTimeMS: a.settings.Verifier.MoveTimeMS}
	}
	timeout := time.Duration(a.settings.LLM.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	return engine.Options{
		Mode:          a.settings.Engine.DefaultMode,
		Personality:   a.settings.Engine.Personality,
		Provider:      provider,
		Verifier:      verify,
		Model:         a.settings.LLM.Model,
		Temperature:   a.settings.LLM.Temperature,
		MaxTokens:     a.settings.LLM.MaxTokens,
		MaxCandidates: a.settings.Engine.MaxCandidates,
		MoveTimeout:   timeout,
	}
}

func (a *Application) NewGame(ctx context.Context, opts engine.NewGameOptions) (*engine.GameState, *AppError) {
	if opts.Mode == "" {
		opts.Mode = a.settings.Engine.DefaultMode
	}
	if opts.Personality == "" {
		opts.Personality = a.settings.Engine.Personality
	}
	state, err := a.engine.NewGame(ctx, opts)
	return state, appErr("ERR_NEW_GAME", err, true)
}

func (a *Application) GetGame(ctx context.Context) (*engine.GameState, *AppError) {
	state, err := a.engine.State(ctx)
	return state, appErr("ERR_GAME_STATE", err, true)
}

func (a *Application) LegalMoves(ctx context.Context) (any, *AppError) {
	moves, err := a.engine.LegalMoves(ctx)
	return moves, appErr("ERR_LEGAL_MOVES", err, true)
}

func (a *Application) MakeUserMove(ctx context.Context, moveUCI string) (*engine.GameState, *AppError) {
	state, err := a.engine.ApplyUserMove(ctx, moveUCI)
	return state, appErr("ERR_INVALID_MOVE", err, true)
}

func (a *Application) RequestEngineMove(ctx context.Context) (any, *AppError) {
	dec, state, err := a.engine.ChooseMove(ctx)
	if err == nil {
		_ = a.traces.AppendDecision(context.Background(), dec)
	}
	return map[string]any{"decision": dec, "state": state}, appErr("ERR_ENGINE_MOVE", err, true)
}

func (a *Application) StopEngine(ctx context.Context) *AppError {
	return appErr("ERR_CANCELLED", a.engine.Stop(ctx), true)
}

func (a *Application) Undo(ctx context.Context, plies int) (*engine.GameState, *AppError) {
	state, err := a.engine.Undo(ctx, plies)
	return state, appErr("ERR_UNDO", err, true)
}

func (a *Application) ExportPGN(ctx context.Context) (string, *AppError) {
	pgn, err := a.engine.ExportPGN(ctx)
	return pgn, appErr("ERR_EXPORT_PGN", err, true)
}

func (a *Application) ExportFEN(ctx context.Context) (string, *AppError) {
	fen, err := a.engine.ExportFEN(ctx)
	return fen, appErr("ERR_EXPORT_FEN", err, true)
}

func (a *Application) GetSettings(ctx context.Context) (storage.Settings, *AppError) {
	settings := a.settings
	if settings.LLM.APIKey != "" {
		settings.LLM.APIKey = "[REDACTED]"
	}
	return settings, nil
}

func (a *Application) SaveSettings(ctx context.Context, settings storage.Settings) *AppError {
	if settings.LLM.APIKey == "[REDACTED]" {
		settings.LLM.APIKey = a.settings.LLM.APIKey
	}
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return appErr("ERR_SETTINGS_INVALID", err, true)
	}
	a.settings = settings
	a.engine.SetOptions(a.engineOptions())
	a.traces = storage.NewTraceStore(settings.Logging.OutputDir)
	return nil
}

func (a *Application) HealthCheckProvider(ctx context.Context) (map[string]any, *AppError) {
	provider := a.engineOptions().Provider
	healthCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := provider.HealthCheck(healthCtx)
	return map[string]any{
		"provider":     provider.Name(),
		"healthy":      err == nil,
		"capabilities": provider.Capabilities(),
	}, appErr("ERR_PROVIDER_HEALTH", err, true)
}

func (a *Application) RunRandomBenchmark(ctx context.Context, games int, seed int64) (experiments.Summary, *AppError) {
	if games <= 0 {
		games = 100
	}
	runner := experiments.Runner{Options: a.engineOptions()}
	summary, err := runner.RandomLegalBenchmark(ctx, games, seed)
	return summary, appErr("ERR_EXPERIMENT", err, true)
}

func (a *Application) Modes(ctx context.Context) []strategy.EngineMode {
	return []strategy.EngineMode{strategy.ModePure, strategy.ModeBlunderguard, strategy.ModeHybrid, strategy.ModeCoach}
}
