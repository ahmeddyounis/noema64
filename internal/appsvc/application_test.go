package appsvc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ahmedyounis/noema64/internal/storage"
)

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
	app := NewApplication("")
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

func TestSaveSettingsKeepsNormalizedRuntimeSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	app := NewApplication(path)

	if appErr := app.SaveSettings(storage.Settings{}); appErr != nil {
		t.Fatalf("save settings: %v", appErr)
	}

	if app.settings.Engine.MaxCandidates == 0 {
		t.Fatal("runtime settings kept unnormalized max_candidates")
	}
	if app.settings.Logging.OutputDir == "" {
		t.Fatal("runtime settings kept unnormalized logging output dir")
	}
}

func TestRequestEngineMoveHonorsTraceEnabled(t *testing.T) {
	traceDir := filepath.Join(t.TempDir(), "traces")
	app := NewApplication("")
	app.settings.Engine.TraceEnabled = false
	app.settings.Logging.OutputDir = traceDir
	app.traces = storage.NewTraceStore(traceDir)

	_, appErr := app.RequestEngineMove()
	if appErr != nil {
		t.Fatalf("engine move: %v", appErr)
	}
	if entries, err := os.ReadDir(traceDir); err == nil && len(entries) > 0 {
		t.Fatalf("trace files written while trace_enabled=false: %v", entries)
	}
}
