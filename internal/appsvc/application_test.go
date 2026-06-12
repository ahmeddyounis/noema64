package appsvc

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

func newTestApplication(t *testing.T) (*Application, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	settings := storage.DefaultSettings()
	settings.Logging.OutputDir = filepath.Join(dir, "logs")
	if err := storage.SaveSettings(configPath, settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	return NewApplication(configPath), settings.Logging.OutputDir
}

func TestWailsBoundMethodsDoNotExposeContextArguments(t *testing.T) {
	appType := reflect.TypeOf(&Application{})
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	for i := 0; i < appType.NumMethod(); i++ {
		method := appType.Method(i)
		for arg := 1; arg < method.Type.NumIn(); arg++ {
			if method.Type.In(arg).String() == "context.Context" {
				t.Fatalf("%s exposes context.Context to Wails", method.Name)
			}
		}
		if method.Type.NumOut() == 2 && !method.Type.Out(1).Implements(errorType) {
			t.Fatalf("%s second return must be error, got %s", method.Name, method.Type.Out(1))
		}
	}
}

func TestImportFENAndPGN(t *testing.T) {
	app, _ := newTestApplication(t)
	fenState, appErr := app.ImportFEN("8/P7/8/8/8/8/8/4k2K w - - 0 1")
	if appErr != nil {
		t.Fatalf("import fen: %v", appErr)
	}
	if fenState.Snapshot.FEN == "" || fenState.Snapshot.SideToMove != "white" {
		t.Fatalf("unexpected fen state: %+v", fenState.Snapshot)
	}

	pgnState, appErr := app.ImportPGN("1. e4 e5 2. Nf3 Nc6 *")
	if appErr != nil {
		t.Fatalf("import pgn: %v", appErr)
	}
	if len(pgnState.Snapshot.MoveHistory) != 4 {
		t.Fatalf("history length = %d, want 4", len(pgnState.Snapshot.MoveHistory))
	}
}

func TestImportRejectsOversizedInput(t *testing.T) {
	app, _ := newTestApplication(t)
	if _, err := app.ImportFEN(strings.Repeat("8/", maxFENImportBytes)); err == nil {
		t.Fatal("expected oversized FEN to fail")
	}
	if _, err := app.ImportPGN(strings.Repeat("1. e4 e5 ", maxPGNImportBytes/9+2)); err == nil {
		t.Fatal("expected oversized PGN to fail")
	}
}

func TestSaveSettingsKeepsNormalizedRuntimeSettings(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := storage.Settings{}
	settings.Logging.OutputDir = filepath.Join(t.TempDir(), "logs")

	if appErr := app.SaveSettings(settings); appErr != nil {
		t.Fatalf("save settings: %v", appErr)
	}

	if app.settings.Engine.MaxCandidates == 0 {
		t.Fatal("runtime settings kept unnormalized max_candidates")
	}
	if app.settings.Logging.OutputDir == "" {
		t.Fatal("runtime settings kept unnormalized logging output dir")
	}
}

func TestSaveSettingsRequiresCloudProviderAcknowledgement(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = "http://localhost:11434/v1"
	settings.Privacy.CloudProviderWarningAcknowledged = false
	if err := app.SaveSettings(settings); err == nil {
		t.Fatal("expected cloud provider settings without acknowledgement to fail")
	}
	settings.Privacy.CloudProviderWarningAcknowledged = true
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save acknowledged cloud provider settings: %v", err)
	}
}

func TestSaveSettingsStoresSelectedProviderProfile(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.Privacy.CloudProviderWarningAcknowledged = true
	settings.LLM.ProfileID = "local-test"
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = "http://localhost:11434/v1"
	settings.LLM.Model = "llama3.1"
	settings.LLM.Temperature = 0.3
	settings.LLM.MaxTokens = 1000
	settings.LLM.TimeoutMS = 8000
	settings.LLM.Retries = 1
	settings.LLM.Profiles = []storage.ProviderProfile{{
		ID:          "local-test",
		Provider:    "openai_compatible",
		Endpoint:    "http://localhost:11434/v1",
		Model:       "llama3.1",
		Temperature: 0.3,
		MaxTokens:   1000,
		TimeoutMS:   8000,
		Retries:     1,
	}}
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save profile settings: %v", err)
	}
	if app.settings.LLM.ProfileID != "local-test" {
		t.Fatalf("profile id = %q, want local-test", app.settings.LLM.ProfileID)
	}
	if app.settings.LLM.Provider != "openai_compatible" || app.settings.LLM.Endpoint != "http://localhost:11434/v1" || app.settings.LLM.Model != "llama3.1" {
		t.Fatalf("selected provider profile not stored: %+v", app.settings.LLM)
	}
}

func TestSaveSettingsRequiresCloudAcknowledgementForSelectedProfile(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.Privacy.CloudProviderWarningAcknowledged = false
	settings.LLM.ProfileID = "local-test"
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = "http://localhost:11434/v1"
	settings.LLM.Model = "llama3.1"
	settings.LLM.MaxTokens = 1000
	settings.LLM.TimeoutMS = 8000
	settings.LLM.Profiles = []storage.ProviderProfile{{
		ID:        "local-test",
		Provider:  "openai_compatible",
		Endpoint:  "http://localhost:11434/v1",
		Model:     "llama3.1",
		MaxTokens: 1000,
		TimeoutMS: 8000,
	}}
	if err := app.SaveSettings(settings); err == nil {
		t.Fatal("expected selected cloud-compatible provider profile without acknowledgement to fail")
	}
}

func TestGetSettingsRedactsProviderProfileKeys(t *testing.T) {
	app, _ := newTestApplication(t)
	app.settings.LLM.Profiles = []storage.ProviderProfile{{
		ID:        "cloud",
		Provider:  "openai_compatible",
		Endpoint:  "https://api.example.test/v1",
		Model:     "model",
		APIKey:    "profile-secret",
		MaxTokens: 1000,
		TimeoutMS: 8000,
	}}
	settings, err := app.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if settings.LLM.Profiles[0].APIKey != "[REDACTED]" {
		t.Fatalf("profile api key was not redacted: %+v", settings.LLM.Profiles[0])
	}
	settings.Privacy.CloudProviderWarningAcknowledged = true
	settings.LLM.ProfileID = "custom"
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save redacted profile settings: %v", err)
	}
	if app.settings.LLM.Profiles[0].APIKey != "profile-secret" {
		t.Fatalf("profile api key was not preserved: %+v", app.settings.LLM.Profiles[0])
	}
}

func TestRequestEngineMoveHonorsTraceEnabled(t *testing.T) {
	app, traceDir := newTestApplication(t)
	app.settings.Engine.TraceEnabled = false
	app.settings.Logging.OutputDir = traceDir
	app.traces = storage.NewTraceStore(traceDir)
	app.games = storage.NewGameStore(filepath.Join(traceDir, "games"))

	_, appErr := app.RequestEngineMove()
	if appErr != nil {
		t.Fatalf("engine move: %v", appErr)
	}
	if entries, err := filepath.Glob(filepath.Join(traceDir, "*.jsonl")); err == nil && len(entries) > 0 {
		t.Fatalf("trace files written while trace_enabled=false: %v", entries)
	}
}

func TestPrivacySettingsEnableRawProviderTrace(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.Privacy.LogRawPrompts = true
	settings.Privacy.LogRawLLMResponses = true
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save raw logging settings: %v", err)
	}
	_, appErr := app.RequestEngineMove()
	if appErr != nil {
		t.Fatalf("engine move: %v", appErr)
	}
	state, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if state.LastDecision == nil {
		t.Fatal("missing last decision")
	}
	if state.LastDecision.Provider.RawPrompt == nil || state.LastDecision.Provider.RawResponse == "" {
		t.Fatalf("raw provider trace not populated: %+v", state.LastDecision.Provider)
	}
}

func TestRunModeBenchmarkCoversCoreModes(t *testing.T) {
	app, _ := newTestApplication(t)
	summary, err := app.RunModeBenchmark(1, 64)
	if err != nil {
		t.Fatalf("run mode benchmark: %v", err)
	}
	wantModes := []strategy.EngineMode{strategy.ModePure, strategy.ModeBlunderguard, strategy.ModeHybrid}
	if len(summary.Results) != len(wantModes) {
		t.Fatalf("results = %d, want %d", len(summary.Results), len(wantModes))
	}
	for i, want := range wantModes {
		result := summary.Results[i]
		if result.Mode != want {
			t.Fatalf("result %d mode = %s, want %s", i, result.Mode, want)
		}
		if result.Summary.GamesCompleted != 1 {
			t.Fatalf("%s completed = %d, want 1", result.Mode, result.Summary.GamesCompleted)
		}
	}
}

func TestNewGameAcceptsTimeControl(t *testing.T) {
	app, _ := newTestApplication(t)
	state, err := app.NewGame(engine.NewGameOptions{
		Side:        "white",
		TimeControl: engine.TimeControl{InitialMS: 60000, IncrementMS: 1000},
	})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	if !state.Clock.Enabled || state.Clock.WhiteMS != 60000 || state.Clock.BlackMS != 60000 || state.Clock.IncrementMS != 1000 {
		t.Fatalf("unexpected clock: %+v", state.Clock)
	}
	restored := NewApplication(filepath.Join(filepath.Dir(app.settingsPath), "config.yaml"))
	restoredState, err := restored.GetGame()
	if err != nil {
		t.Fatalf("restored state: %v", err)
	}
	if restoredState.Clock != state.Clock {
		t.Fatalf("clock did not persist: got %+v want %+v", restoredState.Clock, state.Clock)
	}
}

func TestResignPersistsTerminalOutcome(t *testing.T) {
	app, _ := newTestApplication(t)
	state, err := app.NewGame(engine.NewGameOptions{Side: "white"})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	state, err = app.Resign("white")
	if err != nil {
		t.Fatalf("resign: %v", err)
	}
	if state.Snapshot.Outcome.Status != "resignation" || state.Snapshot.Outcome.Winner != "black" {
		t.Fatalf("unexpected resignation outcome: %+v", state.Snapshot.Outcome)
	}

	restored := NewApplication(filepath.Join(filepath.Dir(app.settingsPath), "config.yaml"))
	restoredState, err := restored.GetGame()
	if err != nil {
		t.Fatalf("restored state: %v", err)
	}
	if restoredState.Snapshot.GameID != state.Snapshot.GameID {
		t.Fatalf("game id = %s, want %s", restoredState.Snapshot.GameID, state.Snapshot.GameID)
	}
	if restoredState.Snapshot.Outcome.Status != "resignation" || restoredState.Snapshot.Outcome.Winner != "black" {
		t.Fatalf("restored outcome = %+v, want black resignation win", restoredState.Snapshot.Outcome)
	}
}

func TestApplicationRestoresLatestGameAndRecentRecords(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	settings := storage.DefaultSettings()
	settings.Logging.OutputDir = filepath.Join(dir, "logs")
	if err := storage.SaveSettings(configPath, settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	app := NewApplication(configPath)
	if _, err := app.NewGame(engine.NewGameOptions{Side: "white"}); err != nil {
		t.Fatalf("new game: %v", err)
	}
	if _, err := app.MakeUserMove("e2e4"); err != nil {
		t.Fatalf("user move: %v", err)
	}
	if _, err := app.RequestEngineMove(); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	before, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if len(before.Snapshot.MoveHistory) < 2 {
		t.Fatalf("expected saved game with engine reply, got %d plies", len(before.Snapshot.MoveHistory))
	}

	restored := NewApplication(configPath)
	after, err := restored.GetGame()
	if err != nil {
		t.Fatalf("restored get game: %v", err)
	}
	if after.Snapshot.GameID != before.Snapshot.GameID {
		t.Fatalf("game id = %s, want %s", after.Snapshot.GameID, before.Snapshot.GameID)
	}
	if len(after.Snapshot.MoveHistory) != len(before.Snapshot.MoveHistory) {
		t.Fatalf("restored plies = %d, want %d", len(after.Snapshot.MoveHistory), len(before.Snapshot.MoveHistory))
	}
	if after.StrategyMemory.SchemaVersion == "" || after.StrategyMemory.LastUpdate.MovePlayed == "" {
		t.Fatalf("strategy memory was not restored: %+v", after.StrategyMemory)
	}
	recent, err := restored.RecentGames(5)
	if err != nil {
		t.Fatalf("recent games: %v", err)
	}
	if len(recent) != 1 || recent[0].GameID != before.Snapshot.GameID {
		t.Fatalf("unexpected recent games: %+v", recent)
	}
	loaded, err := restored.LoadRecentGame(before.Snapshot.GameID)
	if err != nil {
		t.Fatalf("load recent game: %v", err)
	}
	if loaded.Snapshot.GameID != before.Snapshot.GameID || len(loaded.Snapshot.MoveHistory) != len(before.Snapshot.MoveHistory) {
		t.Fatalf("loaded game mismatch: %+v", loaded.Snapshot)
	}
}
