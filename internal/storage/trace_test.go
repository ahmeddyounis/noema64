package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

func TestTraceStoreWritesVersionedRedactedDecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	trace := &decision.MoveDecision{
		SchemaVersion:   "decision-trace.v1",
		DecisionID:      "dec_test",
		GameID:          "game_test",
		Ply:             7,
		Mode:            strategy.ModeBlunderguard,
		SelectedMove:    chesscore.LegalMove{UCI: "g1f3"},
		FENBefore:       "startpos",
		LegalMovesCount: 20,
		Provider: decision.ProviderTrace{
			Name:          "mock",
			Model:         "mock-balanced",
			PromptVersion: strategy.PromptVersion,
			ParseStatus:   "ok",
			RawAvailable:  false,
			Error:         "api_key: abc123",
		},
		Timing:        decision.Timing{TotalMS: 10, ProviderMS: 6, VerifierMS: 3, SearchMS: 1},
		VerifierTrace: &verifier.Result{Enabled: true, Used: true, Name: "static_safety"},
		AnalysisOnly:  true,
		Stages: []decision.StageTrace{{
			Name:       "asking_provider",
			Status:     "completed",
			DurationMS: 6,
		}},
		Assistance: decision.AssistanceTrace{
			Mode:         strategy.ModeBlunderguard,
			VerifierUsed: true,
			VerifierName: "static_safety",
			SearchUsed:   true,
			SearchName:   "deterministic_2ply_material",
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
		SchemaVersion   string              `json:"schema_version"`
		EventType       string              `json:"event_type"`
		EngineVersion   string              `json:"engine_version"`
		GameID          string              `json:"game_id"`
		Ply             int                 `json:"ply"`
		FENBefore       string              `json:"fen_before"`
		LegalMovesCount int                 `json:"legal_moves_count"`
		Mode            strategy.EngineMode `json:"mode"`
		Provider        string              `json:"provider"`
		Model           string              `json:"model"`
		PromptVersion   string              `json:"prompt_version"`
		LLMParseStatus  string              `json:"llm_parse_status"`
		SelectedMove    string              `json:"selected_move"`
		AnalysisOnly    bool                `json:"analysis_only"`
		FallbackUsed    bool                `json:"fallback_used"`
		TimingMS        map[string]int64    `json:"timing_ms"`
		Stages          []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			DurationMS int64  `json:"duration_ms"`
		} `json:"stages"`
		VerifierResult struct {
			Used bool   `json:"used"`
			Name string `json:"name"`
		} `json:"verifier_result"`
		Trace struct {
			SchemaVersion string `json:"schema_version"`
			Assistance    struct {
				VerifierUsed bool   `json:"verifier_used"`
				VerifierName string `json:"verifier_name"`
				SearchUsed   bool   `json:"search_used"`
				SearchName   string `json:"search_name"`
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
	if record.EngineVersion != "0.1.0" || record.GameID != "game_test" || record.Ply != 7 {
		t.Fatalf("missing top-level trace identity fields: %+v", record)
	}
	if record.FENBefore != "startpos" || record.LegalMovesCount != 20 || record.SelectedMove != "g1f3" {
		t.Fatalf("missing top-level position fields: %+v", record)
	}
	if !record.AnalysisOnly {
		t.Fatalf("missing analysis-only disclosure: %+v", record)
	}
	if record.Mode != strategy.ModeBlunderguard || record.Provider != "mock" || record.Model != "mock-balanced" || record.PromptVersion != strategy.PromptVersion {
		t.Fatalf("missing DATA-005 provider metadata: %+v", record)
	}
	if record.LLMParseStatus != "ok" || record.TimingMS["llm"] != 6 || record.TimingMS["verifier"] != 3 || record.TimingMS["search"] != 1 {
		t.Fatalf("missing top-level LLM/timing fields: %+v", record)
	}
	if len(record.Stages) != 1 || record.Stages[0].Name != "asking_provider" || record.Stages[0].DurationMS != 6 {
		t.Fatalf("missing top-level stage timing: %+v", record.Stages)
	}
	if !record.VerifierResult.Used || record.VerifierResult.Name != "static_safety" {
		t.Fatalf("missing top-level verifier disclosure: %+v", record.VerifierResult)
	}
	if record.Trace.SchemaVersion != "decision-trace.v1" {
		t.Fatalf("trace schema version = %q", record.Trace.SchemaVersion)
	}
	if !record.Trace.Assistance.VerifierUsed || record.Trace.Assistance.VerifierName != "static_safety" {
		t.Fatalf("missing assistance disclosure: %+v", record.Trace.Assistance)
	}
	if !record.Trace.Assistance.SearchUsed || record.Trace.Assistance.SearchName != "deterministic_2ply_material" {
		t.Fatalf("missing search disclosure: %+v", record.Trace.Assistance)
	}
	if !record.Trace.VerifierTrace.Used || record.Trace.VerifierTrace.Name != "static_safety" {
		t.Fatalf("missing verifier trace disclosure: %+v", record.Trace.VerifierTrace)
	}
}
