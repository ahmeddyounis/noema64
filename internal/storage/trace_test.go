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
	memBefore := strategy.NewMemory("game_test", "white")
	memAfter := memBefore
	memAfter.Ply = 7
	memAfter.LastUpdate.MovePlayed = "g1f3"
	trace := &decision.MoveDecision{
		SchemaVersion:   "decision-trace.v1",
		DecisionID:      "dec_test",
		GameID:          "game_test",
		Ply:             7,
		Mode:            strategy.ModeBlunderguard,
		SelectedMove:    chesscore.LegalMove{UCI: "g1f3"},
		FENBefore:       "startpos",
		LegalMovesCount: 20,
		StrategyBefore:  memBefore,
		StrategyAfter:   memAfter,
		CandidateMoves: []strategy.CandidateMove{{
			UCI:                "g1f3",
			LLMConfidence:      0.8,
			PlanAlignmentScore: 0.2,
			SearchScore:        0.3,
			PersonalityScore:   0.1,
			FinalScore:         0.7,
			VerifierScore:      strategy.VerifierScore{Status: "accepted", Reason: "test"},
		}},
		Provider: decision.ProviderTrace{
			Name:                  "mock",
			Model:                 "mock-balanced",
			PromptID:              strategy.PromptID,
			PromptVersion:         strategy.PromptVersion,
			PromptSchemaVersion:   strategy.PromptTemplateSchemaVersion,
			DecisionSchemaVersion: strategy.DecisionSchemaVersion,
			Temperature:           0.2,
			MaxTokens:             1600,
			RetryCount:            2,
			ParseStatus:           "ok",
			RawAvailable:          false,
			Error:                 "api_key: abc123",
			RawPrompt:             &decision.PromptTrace{System: "system prompt with api_key: raw-secret", User: "user prompt"},
			RawResponse:           `{"candidate_moves":[{"uci":"g1f3"}],"api_key":"raw-secret"}`,
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
		SchemaVersion      string              `json:"schema_version"`
		EventType          string              `json:"event_type"`
		EngineVersion      string              `json:"engine_version"`
		GameID             string              `json:"game_id"`
		Ply                int                 `json:"ply"`
		FENBefore          string              `json:"fen_before"`
		LegalMovesCount    int                 `json:"legal_moves_count"`
		Mode               strategy.EngineMode `json:"mode"`
		Provider           string              `json:"provider"`
		Model              string              `json:"model"`
		PromptID           string              `json:"prompt_id"`
		PromptVersion      string              `json:"prompt_version"`
		PromptSchema       string              `json:"prompt_schema_version"`
		DecisionSchema     string              `json:"decision_schema_version"`
		Temperature        float64             `json:"temperature"`
		MaxTokens          int                 `json:"max_tokens"`
		RetryCount         int                 `json:"retry_count"`
		LLMParseStatus     string              `json:"llm_parse_status"`
		SelectedMove       string              `json:"selected_move"`
		AnalysisOnly       bool                `json:"analysis_only"`
		FallbackUsed       bool                `json:"fallback_used"`
		StrategyBeforeHash string              `json:"strategy_before_hash"`
		StrategyAfterHash  string              `json:"strategy_after_hash"`
		CandidateMoves     []struct {
			UCI                string  `json:"uci"`
			LLMConfidence      float64 `json:"confidence"`
			PlanAlignmentScore float64 `json:"plan_alignment_score"`
			SearchScore        float64 `json:"search_score"`
			PersonalityScore   float64 `json:"personality_score"`
			FinalScore         float64 `json:"final_score"`
			VerifierScore      struct {
				Status string `json:"status"`
			} `json:"verifier_score"`
		} `json:"candidate_moves"`
		TimingMS map[string]int64 `json:"timing_ms"`
		Stages   []struct {
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
	if len(record.StrategyBeforeHash) != 64 || len(record.StrategyAfterHash) != 64 || record.StrategyBeforeHash == record.StrategyAfterHash {
		t.Fatalf("missing or unchanged strategy memory hashes: before=%q after=%q", record.StrategyBeforeHash, record.StrategyAfterHash)
	}
	if len(record.CandidateMoves) != 1 || record.CandidateMoves[0].UCI != "g1f3" {
		t.Fatalf("missing top-level candidate moves: %+v", record.CandidateMoves)
	}
	candidate := record.CandidateMoves[0]
	if candidate.LLMConfidence != 0.8 || candidate.PlanAlignmentScore != 0.2 || candidate.SearchScore != 0.3 || candidate.PersonalityScore != 0.1 || candidate.FinalScore != 0.7 || candidate.VerifierScore.Status != "accepted" {
		t.Fatalf("missing candidate scoring components: %+v", candidate)
	}
	if !record.AnalysisOnly {
		t.Fatalf("missing analysis-only disclosure: %+v", record)
	}
	if record.Mode != strategy.ModeBlunderguard || record.Provider != "mock" || record.Model != "mock-balanced" || record.PromptID != strategy.PromptID || record.PromptVersion != strategy.PromptVersion {
		t.Fatalf("missing DATA-005 provider metadata: %+v", record)
	}
	if record.PromptSchema != strategy.PromptTemplateSchemaVersion || record.DecisionSchema != strategy.DecisionSchemaVersion {
		t.Fatalf("missing prompt/schema replay metadata: %+v", record)
	}
	if record.Temperature != 0.2 || record.MaxTokens != 1600 || record.RetryCount != 2 {
		t.Fatalf("missing provider runtime metadata: %+v", record)
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

	exported, err := NewTraceFileStore(path).ReadGame(context.Background(), "")
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	if strings.Contains(exported, "abc123") || !strings.Contains(exported, `"event_type":"move_decision"`) {
		t.Fatalf("exported trace not redacted or incomplete: %s", exported)
	}
	if strings.Contains(exported, "raw_prompt") || strings.Contains(exported, "raw_response") || strings.Contains(exported, "raw-secret") {
		t.Fatalf("normal trace export kept raw provider data: %s", exported)
	}
	debugExported, err := NewTraceFileStore(path).ReadGameDebug(context.Background(), "")
	if err != nil {
		t.Fatalf("read debug trace file: %v", err)
	}
	if !strings.Contains(debugExported, "raw_prompt") || !strings.Contains(debugExported, "raw_response") {
		t.Fatalf("debug trace export stripped raw provider data: %s", debugExported)
	}
	if strings.Contains(debugExported, "raw-secret") {
		t.Fatalf("debug trace export leaked unredacted secret: %s", debugExported)
	}
}

func TestTraceStoreSanitizesGameIDFileNames(t *testing.T) {
	dir := t.TempDir()
	traceDir := filepath.Join(dir, "logs")
	outsidePath := filepath.Join(dir, "secret.jsonl")
	if err := os.WriteFile(outsidePath, []byte("outside-only\n"), 0o600); err != nil {
		t.Fatalf("write outside trace: %v", err)
	}
	store := NewTraceStore(traceDir)
	if text, err := store.ReadGame(context.Background(), "../secret"); err == nil {
		t.Fatalf("traversal-shaped game id read outside trace dir: %q", text)
	}
	trace := &decision.MoveDecision{
		GameID:       "../secret",
		SelectedMove: chesscore.LegalMove{UCI: "e2e4"},
	}
	if err := store.AppendDecision(context.Background(), trace); err != nil {
		t.Fatalf("append sanitized trace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(traceDir, "secret.jsonl")); err != nil {
		t.Fatalf("sanitized trace was not written inside trace dir: %v", err)
	}
	outside, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatalf("read outside trace: %v", err)
	}
	if string(outside) != "outside-only\n" {
		t.Fatalf("outside trace was modified: %q", outside)
	}
}
