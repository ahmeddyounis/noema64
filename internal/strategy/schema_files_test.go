package strategy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublicSchemaFilesTrackStrategyVersions(t *testing.T) {
	root := repoRoot(t)
	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "move decision", file: "move_decision.schema.json", want: DecisionSchemaVersion},
		{name: "strategy memory", file: "strategy_memory.schema.json", want: MemorySchemaVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(root, "schemas", tt.file))
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			var schema struct {
				Properties map[string]struct {
					Const string `json:"const"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(b, &schema); err != nil {
				t.Fatalf("schema is not valid JSON: %v", err)
			}
			if got := schema.Properties["schema_version"].Const; got != tt.want {
				t.Fatalf("schema_version const = %q, want %q", got, tt.want)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
