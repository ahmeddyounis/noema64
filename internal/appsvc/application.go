package appsvc

import (
	"context"
	"strings"
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

const (
	maxFENImportBytes = 512
	maxPGNImportBytes = 1 << 20
)

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
		provider = providers.OpenAICompatible{BaseURL: a.settings.LLM.Endpoint, APIKey: a.settings.LLM.APIKey, Model: a.settings.LLM.Model}
	}
	var verify verifier.Verifier = verifier.StaticVerifier{Enabled: a.settings.Verifier.Enabled}
	if a.settings.Verifier.Enabled && a.settings.Verifier.Path != "" {
		verify = verifier.ExternalUCI{
			Path:             a.settings.Verifier.Path,
			MoveTimeMS:       a.settings.Verifier.MoveTimeMS,
			MaxCentipawnLoss: a.settings.Verifier.MaxCentipawnLoss,
		}
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

func (a *Application) NewGame(opts engine.NewGameOptions) (*engine.GameState, error) {
	if opts.Mode == "" {
		opts.Mode = a.settings.Engine.DefaultMode
	}
	if opts.Personality == "" {
		opts.Personality = a.settings.Engine.Personality
	}
	state, err := a.engine.NewGame(context.Background(), opts)
	return state, appErr("ERR_NEW_GAME", err, true)
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
	return state, appErr("ERR_INVALID_MOVE", err, true)
}

func (a *Application) RequestEngineMove() (any, error) {
	dec, state, err := a.engine.ChooseMove(context.Background())
	if err == nil && a.settings.Engine.TraceEnabled {
		_ = a.traces.AppendDecision(context.Background(), dec)
	}
	return map[string]any{"decision": dec, "state": state}, appErr("ERR_ENGINE_MOVE", err, true)
}

func (a *Application) StopEngine() error {
	return appErr("ERR_CANCELLED", a.engine.Stop(context.Background()), true)
}

func (a *Application) Undo(plies int) (*engine.GameState, error) {
	state, err := a.engine.Undo(context.Background(), plies)
	return state, appErr("ERR_UNDO", err, true)
}

func (a *Application) ExportPGN() (string, error) {
	pgn, err := a.engine.ExportPGN(context.Background())
	return pgn, appErr("ERR_EXPORT_PGN", err, true)
}

func (a *Application) ExportFEN() (string, error) {
	fen, err := a.engine.ExportFEN(context.Background())
	return fen, appErr("ERR_EXPORT_FEN", err, true)
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
	return state, appErr("ERR_IMPORT_INVALID_FEN", err, true)
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
	return state, appErr("ERR_IMPORT_INVALID_PGN", err, true)
}

func (a *Application) GetSettings() (storage.Settings, error) {
	settings := a.settings
	if settings.LLM.APIKey != "" {
		settings.LLM.APIKey = "[REDACTED]"
	}
	return settings, nil
}

func (a *Application) SaveSettings(settings storage.Settings) error {
	if settings.LLM.APIKey == "[REDACTED]" {
		settings.LLM.APIKey = a.settings.LLM.APIKey
	}
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return appErr("ERR_SETTINGS_INVALID", err, true)
	}
	settings = storage.NormalizeSettings(settings)
	a.settings = settings
	a.engine.SetOptions(a.engineOptions())
	a.traces = storage.NewTraceStore(settings.Logging.OutputDir)
	return nil
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

func (a *Application) Modes() []strategy.EngineMode {
	return []strategy.EngineMode{strategy.ModePure, strategy.ModeBlunderguard, strategy.ModeHybrid, strategy.ModeCoach}
}
