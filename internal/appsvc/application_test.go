package appsvc

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/analysis"
	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/experiments"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
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

func TestImportErrorsDoNotMutateCurrentGame(t *testing.T) {
	app, _ := newTestApplication(t)
	before, err := app.GetGame()
	if err != nil {
		t.Fatalf("get before: %v", err)
	}
	if _, err := app.ImportFEN("not a fen"); err == nil {
		t.Fatal("expected invalid FEN import to fail")
	}
	afterFEN, err := app.GetGame()
	if err != nil {
		t.Fatalf("get after invalid fen: %v", err)
	}
	if afterFEN.Snapshot.GameID != before.Snapshot.GameID || afterFEN.Snapshot.FEN != before.Snapshot.FEN {
		t.Fatalf("invalid FEN import mutated game: before=%+v after=%+v", before.Snapshot, afterFEN.Snapshot)
	}
	if _, err := app.ImportPGN("1. e4 e9 *"); err == nil {
		t.Fatal("expected invalid PGN import to fail")
	}
	afterPGN, err := app.GetGame()
	if err != nil {
		t.Fatalf("get after invalid pgn: %v", err)
	}
	if afterPGN.Snapshot.GameID != before.Snapshot.GameID || afterPGN.Snapshot.FEN != before.Snapshot.FEN {
		t.Fatalf("invalid PGN import mutated game: before=%+v after=%+v", before.Snapshot, afterPGN.Snapshot)
	}
}

func TestAnalyzeCurrentPositionDoesNotMutateCurrentGame(t *testing.T) {
	app, _ := newTestApplication(t)
	before, err := app.GetGame()
	if err != nil {
		t.Fatalf("get before: %v", err)
	}
	dec, err := app.AnalyzeCurrentPosition()
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !dec.AnalysisOnly {
		t.Fatalf("analysis_only = false for decision %+v", dec)
	}
	after, err := app.GetGame()
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if after.Snapshot.Ply != before.Snapshot.Ply || after.Snapshot.FEN != before.Snapshot.FEN {
		t.Fatalf("analysis mutated current game: before=%+v after=%+v", before.Snapshot, after.Snapshot)
	}
}

func TestExportTraceReturnsCurrentGameJSONL(t *testing.T) {
	app, _ := newTestApplication(t)
	if _, err := app.RequestEngineMove(); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	trace, err := app.ExportTrace()
	if err != nil {
		t.Fatalf("export trace: %v", err)
	}
	for _, want := range []string{`"event_type":"move_decision"`, `"schema_version":"1.0"`, `"selected_move"`} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace missing %q:\n%s", want, trace)
		}
	}
}

func TestWhyNotMoveComparesAgainstLastDecision(t *testing.T) {
	app, _ := newTestApplication(t)
	if _, err := app.RequestEngineMove(); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	state, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if state.LastDecision == nil || len(state.LastDecision.CandidateMoves) == 0 {
		t.Fatalf("missing decision candidates: %+v", state.LastDecision)
	}
	candidate := state.LastDecision.CandidateMoves[0].UCI
	comparison, err := app.WhyNotMove(candidate)
	if err != nil {
		t.Fatalf("why not: %v", err)
	}
	if comparison.RequestedMove != candidate || comparison.SelectedMove != state.LastDecision.SelectedMove.UCI || comparison.Summary == "" {
		t.Fatalf("unexpected comparison: %+v", comparison)
	}
}

func TestWhyNotMoveRequiresLastDecision(t *testing.T) {
	app, _ := newTestApplication(t)
	if _, err := app.WhyNotMove("e2e4"); err == nil {
		t.Fatal("expected missing decision comparison to fail")
	}
}

func TestNewApplicationRecoversFromCorruptSettings(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("llm: [not valid yaml\n"), 0o600); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}
	app := NewApplication(configPath)
	if app.settings.SchemaVersion == "" || app.settings.LLM.Provider != "mock" {
		t.Fatalf("app did not recover with defaults: %+v", app.settings)
	}
	state, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game after corrupt config recovery: %v", err)
	}
	if state.Snapshot.FEN == "" {
		t.Fatalf("recovered app has empty game state: %+v", state.Snapshot)
	}
}

func TestNewApplicationReportsUnsupportedFutureSettingsSchema(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("schema_version: \"99.0\"\n"), 0o600); err != nil {
		t.Fatalf("write future config: %v", err)
	}
	app := NewApplication(configPath)
	if _, err := app.GetSettings(); err == nil {
		t.Fatal("expected future settings schema to be reported")
	}
	state, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game should still recover with defaults: %v", err)
	}
	if state.Snapshot.FEN == "" {
		t.Fatalf("recovered app has empty game state: %+v", state.Snapshot)
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

func TestEngineOptionsHonorsProviderRetries(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.Privacy.CloudProviderWarningAcknowledged = true
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = "http://localhost:11434/v1"
	settings.LLM.Model = "llama3.1"
	settings.LLM.Retries = 3
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save retry settings: %v", err)
	}
	provider, ok := app.engineOptions().Provider.(providers.OpenAICompatible)
	if !ok {
		t.Fatalf("provider = %T, want OpenAICompatible", app.engineOptions().Provider)
	}
	if provider.Retries != 3 {
		t.Fatalf("provider retries = %d, want 3", provider.Retries)
	}
	if app.engineOptions().ProviderRetries != 3 {
		t.Fatalf("engine option provider retries = %d, want 3", app.engineOptions().ProviderRetries)
	}
}

func TestEngineOptionsResolvesAPIKeyRef(t *testing.T) {
	t.Setenv("NOEMA64_KEYCHAIN_PROVIDER_CLOUD", "resolved-secret")
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.Privacy.CloudProviderWarningAcknowledged = true
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = "http://localhost:11434/v1"
	settings.LLM.Model = "llama3.1"
	settings.LLM.APIKey = ""
	settings.LLM.APIKeyRef = "provider/cloud"
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save key ref settings: %v", err)
	}
	provider, ok := app.engineOptions().Provider.(providers.OpenAICompatible)
	if !ok {
		t.Fatalf("provider = %T, want OpenAICompatible", app.engineOptions().Provider)
	}
	if provider.APIKey != "resolved-secret" {
		t.Fatalf("provider api key = %q, want resolved keychain secret", provider.APIKey)
	}
}

func TestEngineOptionsWrapsTablebaseVerifier(t *testing.T) {
	app, _ := newTestApplication(t)
	settings := app.settings
	settings.Verifier.TablebaseEnabled = true
	settings.Verifier.TablebasePath = "/usr/local/bin/noema64-tablebase"
	settings.Verifier.TablebaseTimeoutMS = 750
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("save tablebase settings: %v", err)
	}

	tb, ok := app.engineOptions().Verifier.(verifier.TablebaseVerifier)
	if !ok {
		t.Fatalf("verifier = %T, want TablebaseVerifier", app.engineOptions().Verifier)
	}
	probe, ok := tb.Probe.(verifier.ExternalTablebase)
	if !ok || probe.Path != settings.Verifier.TablebasePath || probe.TimeoutMS != 750 {
		t.Fatalf("tablebase probe = %#v, want configured external tablebase", tb.Probe)
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
	trace, err := app.ExportTrace()
	if err != nil {
		t.Fatalf("export trace: %v", err)
	}
	if strings.Contains(trace, "raw_prompt") || strings.Contains(trace, "raw_response") {
		t.Fatalf("normal trace export included raw provider data: %s", trace)
	}
	debugTrace, err := app.ExportDebugTrace()
	if err != nil {
		t.Fatalf("export debug trace: %v", err)
	}
	if !strings.Contains(debugTrace, "raw_prompt") || !strings.Contains(debugTrace, "raw_response") {
		t.Fatalf("debug trace export missing raw provider data: %s", debugTrace)
	}
}

func TestRequestEngineMoveEmitsDecisionStageEvents(t *testing.T) {
	app, _ := newTestApplication(t)
	events := []capturedEvent{}
	app.SetEventSink(func(name string, payload any) {
		events = append(events, capturedEvent{name: name, payload: payload})
	})

	_, appErr := app.RequestEngineMove()
	if appErr != nil {
		t.Fatalf("engine move: %v", appErr)
	}
	state, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected decision stage events")
	}
	if events[0].name != "decision.stage" {
		t.Fatalf("event name = %s, want decision.stage", events[0].name)
	}
	if !hasCapturedStage(events, "playing_move") || !hasCapturedStage(events, "writing_trace") {
		t.Fatalf("missing emitted playing/write stages: %+v", events)
	}
	if state.LastDecision == nil {
		t.Fatal("missing last decision")
	}
	if !hasDecisionStage(state.LastDecision.Stages, "playing_move") || !hasDecisionStage(state.LastDecision.Stages, "writing_trace") {
		t.Fatalf("missing engine/app stages: %+v", state.LastDecision.Stages)
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

func TestPostGameReviewAndStrategyMetrics(t *testing.T) {
	app, _ := newTestApplication(t)
	emptyReview, err := app.PostGameReview()
	if err != nil {
		t.Fatalf("empty review: %v", err)
	}
	if emptyReview.SchemaVersion != postGameReviewSchemaVersion || emptyReview.GameID == "" || emptyReview.StrategyMetrics.SchemaVersion == "" {
		t.Fatalf("empty review missing boundary fields: %+v", emptyReview)
	}

	if _, err := app.RequestEngineMove(); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	metrics, err := app.StrategyMetrics()
	if err != nil {
		t.Fatalf("strategy metrics: %v", err)
	}
	if metrics.SchemaVersion != strategy.MemoryMetricsSchemaVersion || metrics.Quality <= 0 {
		t.Fatalf("metrics not populated: %+v", metrics)
	}
	review, err := app.PostGameReview()
	if err != nil {
		t.Fatalf("post-game review: %v", err)
	}
	if review.SelectedMove == "" || review.CandidateCount == 0 || review.Provider == "" || review.Summary == "" {
		t.Fatalf("review missing decision context: %+v", review)
	}
}

func TestStudyDashboardAndMultiAgentAnalysis(t *testing.T) {
	app, _ := newTestApplication(t)
	initial, err := app.StudyDashboard()
	if err != nil {
		t.Fatalf("initial study dashboard: %v", err)
	}
	if initial.SchemaVersion != studyDashboardSchemaVersion || initial.Memory.SchemaVersion != strategy.CompressedMemorySchemaVersion {
		t.Fatalf("initial dashboard missing schema fields: %+v", initial)
	}
	if initial.MultiAgent.SchemaVersion == "" || initial.Lesson.Title == "" || initial.Puzzle.FEN == "" {
		t.Fatalf("initial dashboard missing study tools: %+v", initial)
	}
	if len(initial.OpeningBook) == 0 || initial.Endgame.Theme == "" {
		t.Fatalf("initial dashboard missing book/endgame data: %+v", initial)
	}

	if _, err := app.RequestEngineMove(); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	dashboard, err := app.StudyDashboard()
	if err != nil {
		t.Fatalf("study dashboard: %v", err)
	}
	if dashboard.CandidateDiversity.CandidateCount == 0 || len(dashboard.MultiAgent.Reviews) < 4 || len(dashboard.Heatmap) == 0 || len(dashboard.Timeline) < 2 {
		t.Fatalf("dashboard missing decision study surfaces: %+v", dashboard)
	}
	multi, err := app.MultiAgentAnalysis()
	if err != nil {
		t.Fatalf("multi-agent analysis: %v", err)
	}
	if multi.Arbiter == "" || !hasAgentRole(multi.Reviews, "tactician") {
		t.Fatalf("multi-agent review incomplete: %+v", multi)
	}
	compressed, err := app.CompressedStrategyMemory(2)
	if err != nil {
		t.Fatalf("compressed memory: %v", err)
	}
	if compressed.RetainedItems == 0 || compressed.SourceHash == "" {
		t.Fatalf("compressed memory incomplete: %+v", compressed)
	}
	coherence, err := app.PlanCoherence()
	if err != nil {
		t.Fatalf("plan coherence: %v", err)
	}
	if coherence.SchemaVersion != strategy.PlanCoherenceSchemaVersion {
		t.Fatalf("bad coherence report: %+v", coherence)
	}
}

func TestRoadmapComparisonAndBuilderAPIs(t *testing.T) {
	app, _ := newTestApplication(t)
	book, err := app.OpeningBook()
	if err != nil {
		t.Fatalf("opening book: %v", err)
	}
	if len(book) == 0 {
		t.Fatalf("expected opening book suggestions")
	}
	comparison, err := app.ComparePureHybridAnalysis()
	if err != nil {
		t.Fatalf("compare pure/hybrid: %v", err)
	}
	if comparison.SchemaVersion == "" || comparison.Pure == nil || comparison.Hybrid == nil || comparison.Summary == "" {
		t.Fatalf("analysis comparison incomplete: %+v", comparison)
	}
	pack, err := app.PromptTemplatePack()
	if err != nil {
		t.Fatalf("prompt pack: %v", err)
	}
	modified := pack
	modified.User += "\nExtra comparison text.\n"
	promptComparison, err := app.ComparePromptTemplatePacks(pack, modified)
	if err != nil {
		t.Fatalf("prompt comparison: %v", err)
	}
	if promptComparison.SchemaVersion == "" || promptComparison.LeftHash == promptComparison.RightHash || !containsString(promptComparison.ChangedFiles, "move_decision.md") {
		t.Fatalf("prompt comparison incomplete: %+v", promptComparison)
	}
	profile, err := app.BuildCustomPersonalityProfile("sharp", "Sharp", 1.5, []string{"initiative", "initiative"}, []string{"Prefer active moves."})
	if err != nil {
		t.Fatalf("build personality: %v", err)
	}
	if profile.ID != "sharp" || profile.RiskTolerance != 1 || len(profile.StrategicBiases) != 1 {
		t.Fatalf("custom profile not normalized: %+v", profile)
	}
}

func TestUpdateStrategyMemoryPersistsEditorChanges(t *testing.T) {
	app, _ := newTestApplication(t)
	state, err := app.GetGame()
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	mem := state.StrategyMemory
	mem.Plan.Summary = "Edited plan from study tools."
	mem.Commitments = []string{"Preserve the edited plan."}
	updated, err := app.UpdateStrategyMemory(mem)
	if err != nil {
		t.Fatalf("update memory: %v", err)
	}
	if updated.StrategyMemory.Plan.Summary != mem.Plan.Summary || updated.StrategyMemory.GameID != state.Snapshot.GameID {
		t.Fatalf("memory update not reflected: %+v", updated.StrategyMemory)
	}

	restored := NewApplication(filepath.Join(filepath.Dir(app.settingsPath), "config.yaml"))
	restoredState, err := restored.GetGame()
	if err != nil {
		t.Fatalf("restored state: %v", err)
	}
	if restoredState.StrategyMemory.Plan.Summary != mem.Plan.Summary {
		t.Fatalf("memory edit did not persist: %+v", restoredState.StrategyMemory)
	}
}

func TestProviderDashboardHonorsProfilePrivacy(t *testing.T) {
	app, _ := newTestApplication(t)
	dashboard, err := app.ProviderDashboard()
	if err != nil {
		t.Fatalf("provider dashboard: %v", err)
	}
	if dashboard.SchemaVersion != providerDashboardSchemaVersion || len(dashboard.Profiles) == 0 {
		t.Fatalf("dashboard missing fields: %+v", dashboard)
	}
	var sawMockHealthy bool
	var sawPrivacyGate bool
	for _, profile := range dashboard.Profiles {
		if profile.Provider == "mock" && profile.Healthy {
			sawMockHealthy = true
		}
		if profile.Provider == "openai_compatible" && profile.Status == "privacy_ack_required" {
			sawPrivacyGate = true
		}
	}
	if !sawMockHealthy || !sawPrivacyGate {
		t.Fatalf("dashboard did not expose expected profile statuses: %+v", dashboard.Profiles)
	}
}

func TestProviderProfileImportExportRedactsAndPreservesKeys(t *testing.T) {
	app, _ := newTestApplication(t)
	app.settings.LLM.Profiles = []storage.ProviderProfile{{
		ID:        "cloud",
		Provider:  "openai_compatible",
		Endpoint:  "https://api.example.test/v1",
		Model:     "model",
		APIKey:    "secret",
		MaxTokens: 1000,
		TimeoutMS: 1000,
	}}
	exported, err := app.ExportProviderProfiles()
	if err != nil {
		t.Fatalf("export provider profiles: %v", err)
	}
	if strings.Contains(exported, "secret") || !strings.Contains(exported, "[REDACTED]") {
		t.Fatalf("profiles were not redacted:\n%s", exported)
	}
	settings, err := app.ImportProviderProfiles(exported)
	if err != nil {
		t.Fatalf("import provider profiles: %v", err)
	}
	if settings.LLM.Profiles[0].APIKey != "[REDACTED]" {
		t.Fatalf("returned settings did not redact key: %+v", settings.LLM.Profiles[0])
	}
	if app.settings.LLM.Profiles[0].APIKey != "secret" {
		t.Fatalf("redacted import did not preserve prior key: %+v", app.settings.LLM.Profiles[0])
	}
}

func TestBackupRestoreFineTuneAndTournamentWorkflows(t *testing.T) {
	app, logDir := newTestApplication(t)
	if _, err := app.RequestEngineMove(); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	backupDir := filepath.Join(filepath.Dir(logDir), "backups")
	manifest, err := app.CreateBackup(backupDir)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if manifest.SchemaVersion != storage.BackupManifestSchemaVersion || manifest.ArchivePath == "" || len(manifest.Files) == 0 {
		t.Fatalf("backup manifest incomplete: %+v", manifest)
	}
	restored, err := app.RestoreBackup(manifest.ArchivePath, filepath.Join(filepath.Dir(logDir), "restore"))
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	if len(restored.Files) != len(manifest.Files) {
		t.Fatalf("restored files = %d, want %d", len(restored.Files), len(manifest.Files))
	}

	workflow, err := app.ExportFineTuneDataset()
	if err != nil {
		t.Fatalf("fine-tune export: %v", err)
	}
	if workflow.Workflow.ExampleCount == 0 || !strings.Contains(workflow.DatasetJSONL, "selected_move") {
		t.Fatalf("fine-tune workflow incomplete: %+v", workflow.Workflow)
	}

	tournament, err := app.RunTournament(1, 64)
	if err != nil {
		t.Fatalf("tournament: %v", err)
	}
	if tournament.SchemaVersion != experiments.TournamentSchemaVersion || tournament.GamesPlayed == 0 || len(tournament.Ratings) < 2 {
		t.Fatalf("tournament summary incomplete: %+v", tournament)
	}
}

func TestExportFineTuneDatasetHandlesNoTrace(t *testing.T) {
	app, _ := newTestApplication(t)
	workflow, err := app.ExportFineTuneDataset()
	if err != nil {
		t.Fatalf("fine-tune export without trace: %v", err)
	}
	if workflow.Workflow.ExampleCount != 0 || strings.TrimSpace(workflow.DatasetJSONL) != "" {
		t.Fatalf("unexpected empty workflow: %+v", workflow)
	}
}

func TestPromptTemplatePackValidationAndSave(t *testing.T) {
	app, _ := newTestApplication(t)
	pack, err := app.PromptTemplatePack()
	if err != nil {
		t.Fatalf("prompt pack: %v", err)
	}
	validation, err := app.ValidatePromptTemplatePack(pack)
	if err != nil {
		t.Fatalf("validate prompt pack: %v", err)
	}
	if !validation.Valid || validation.SchemaVersion != promptTemplateValidationSchemaVersion {
		t.Fatalf("default prompt pack invalid: %+v", validation)
	}
	dir := t.TempDir()
	validation, err = app.SavePromptTemplatePack(dir, pack)
	if err != nil {
		t.Fatalf("save prompt pack: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("saved invalid prompt pack: %+v", validation)
	}
	if _, err := strategy.LoadPromptTemplates(dir); err != nil {
		t.Fatalf("saved prompt pack did not load: %v", err)
	}
	pack.User += "\n{{unknown_placeholder}}\n"
	validation, err = app.ValidatePromptTemplatePack(pack)
	if err != nil {
		t.Fatalf("validate invalid prompt pack: %v", err)
	}
	if validation.Valid || len(validation.Errors) == 0 {
		t.Fatalf("invalid prompt pack accepted: %+v", validation)
	}
}

func TestPositionSuiteAndProviderComparison(t *testing.T) {
	app, _ := newTestApplication(t)
	suite, err := app.RunPositionSuite([]experiments.SuitePosition{{
		Name: "start",
		FEN:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
	}})
	if err != nil {
		t.Fatalf("run position suite: %v", err)
	}
	if suite.SchemaVersion != experiments.PositionSuiteSchemaVersion || suite.PositionsAnalyzed != 1 {
		t.Fatalf("unexpected suite summary: %+v", suite)
	}
	comparison, err := app.RunProviderComparison()
	if err != nil {
		t.Fatalf("run provider comparison: %v", err)
	}
	if comparison.SchemaVersion != providerComparisonSchemaVersion || comparison.ProfilesCompared == 0 {
		t.Fatalf("unexpected provider comparison summary: %+v", comparison)
	}
	var sawPrivacyGate bool
	for _, result := range comparison.Results {
		if result.Status == "privacy_ack_required" {
			sawPrivacyGate = true
		}
	}
	if !sawPrivacyGate {
		t.Fatalf("provider comparison did not gate cloud profiles: %+v", comparison.Results)
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

func TestNewGameAcceptsChess960Variant(t *testing.T) {
	app, _ := newTestApplication(t)
	state, err := app.NewGame(engine.NewGameOptions{Side: "white", Variant: "chess960", Seed: 17})
	if err != nil {
		t.Fatalf("new Chess960 game: %v", err)
	}
	if state.Variant.Variant != "chess960" || state.InitialFEN == "" {
		t.Fatalf("missing Chess960 state metadata: %+v", state.Variant)
	}
	if len(state.Snapshot.LegalMoves) == 0 {
		t.Fatalf("Chess960 start has no legal moves: %+v", state.Snapshot)
	}

	restored := NewApplication(filepath.Join(filepath.Dir(app.settingsPath), "config.yaml"))
	restoredState, err := restored.GetGame()
	if err != nil {
		t.Fatalf("restored state: %v", err)
	}
	if restoredState.Variant.Variant != "chess960" || restoredState.InitialFEN != state.InitialFEN {
		t.Fatalf("Chess960 metadata did not persist: got %+v initial=%s want %+v initial=%s", restoredState.Variant, restoredState.InitialFEN, state.Variant, state.InitialFEN)
	}
}

func TestNewGameAcceptsCustomBoardDefinition(t *testing.T) {
	app, _ := newTestApplication(t)
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
	state, err := app.NewGame(engine.NewGameOptions{Side: "white", Variant: chesscore.VariantCustom, BoardDefinition: &def})
	if err != nil {
		t.Fatalf("new custom game: %v", err)
	}
	if state.Variant.BoardDefinition == nil || !containsAppLegalMove(state.Snapshot.LegalMoves, "d4e6") {
		t.Fatalf("custom game was not playable: variant=%+v moves=%+v", state.Variant, state.Snapshot.LegalMoves)
	}
	state, err = app.MakeUserMove("d4e6")
	if err != nil {
		t.Fatalf("apply custom move: %v", err)
	}
	if state.Snapshot.Board["e6"] != "A" {
		t.Fatalf("custom board did not update: %+v", state.Snapshot.Board)
	}

	restored := NewApplication(filepath.Join(filepath.Dir(app.settingsPath), "config.yaml"))
	restoredState, err := restored.GetGame()
	if err != nil {
		t.Fatalf("restored state: %v", err)
	}
	if restoredState.Snapshot.Board["e6"] != "A" || restoredState.Variant.BoardDefinition == nil {
		t.Fatalf("custom game did not persist: %+v", restoredState)
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

func hasDecisionStage(stages []decision.StageTrace, name string) bool {
	for _, stage := range stages {
		if stage.Name == name {
			return true
		}
	}
	return false
}

func hasAgentRole(reviews []analysis.AgentReview, role string) bool {
	for _, review := range reviews {
		if review.Role == role {
			return true
		}
	}
	return false
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsAppLegalMove(moves []chesscore.LegalMove, want string) bool {
	for _, move := range moves {
		if move.UCI == want {
			return true
		}
	}
	return false
}

type capturedEvent struct {
	name    string
	payload any
}

func hasCapturedStage(events []capturedEvent, stageName string) bool {
	for _, event := range events {
		progress, ok := event.payload.(decision.ProgressEvent)
		if ok && progress.Stage == stageName {
			return true
		}
	}
	return false
}
