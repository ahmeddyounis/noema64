package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

func TestTraceStoreWritesVersionedRedactedDecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	trace := &decision.MoveDecision{
		SchemaVersion: "decision-trace.v1",
		DecisionID:    "dec_test",
		GameID:        "game_test",
		Mode:          strategy.ModeBlunderguard,
		Provider: decision.ProviderTrace{
			Name:  "mock",
			Model: "mock-balanced",
			Error: "api_key: abc123",
		},
		VerifierTrace: &verifier.Result{Enabled: true, Used: true, Name: "static_safety"},
		Assistance: decision.AssistanceTrace{
			Mode:         strategy.ModeBlunderguard,
			VerifierUsed: true,
			VerifierName: "static_safety",
		},
	}

	if err := NewTraceFileStore(path).AppendDecision(context.Background(), trace); err != nil {
		t.Fatalf("append trace: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	if strings.Contains(string(b), "abc123") {
		t.Fatalf("trace leaked secret: %s", string(b))
	}

	var record struct {
		SchemaVersion string `json:"schema_version"`
		EventType     string `json:"event_type"`
		Trace         struct {
			SchemaVersion string `json:"schema_version"`
			Assistance    struct {
				VerifierUsed bool   `json:"verifier_used"`
				VerifierName string `json:"verifier_name"`
			} `json:"assistance"`
			VerifierTrace struct {
				Used bool   `json:"used"`
				Name string `json:"name"`
			} `json:"verifier_trace"`
		} `json:"trace"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &record); err != nil {
		t.Fatalf("unmarshal trace: %v", err)
	}
	if record.SchemaVersion == "" || record.EventType != "move_decision" {
		t.Fatalf("missing record metadata: %+v", record)
	}
	if record.Trace.SchemaVersion != "decision-trace.v1" {
		t.Fatalf("trace schema version = %q", record.Trace.SchemaVersion)
	}
	if !record.Trace.Assistance.VerifierUsed || record.Trace.Assistance.VerifierName != "static_safety" {
		t.Fatalf("missing assistance disclosure: %+v", record.Trace.Assistance)
	}
	if !record.Trace.VerifierTrace.Used || record.Trace.VerifierTrace.Name != "static_safety" {
		t.Fatalf("missing verifier trace disclosure: %+v", record.Trace.VerifierTrace)
	}
}
