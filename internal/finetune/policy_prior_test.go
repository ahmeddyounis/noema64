package finetune

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestTrainPolicyPriorJSONLAndProvider(t *testing.T) {
	example := DatasetExample{
		SchemaVersion: DatasetExampleSchemaVersion,
		DecisionID:    "dec-1",
		Messages: []Message{{
			Role: "user",
			Content: strings.Join([]string{
				"FEN: rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
				"Legal moves count: 20",
			}, "\n"),
		}},
		Output: FineTuneOutput{
			SelectedMove:    "e2e4",
			Explanation:     "Take the center.",
			PositionSummary: "Start position.",
		},
	}
	line, err := json.Marshal(example)
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}
	model, err := TrainPolicyPriorJSONL(string(line) + "\n")
	if err != nil {
		t.Fatalf("train policy prior: %v", err)
	}
	if model.SchemaVersion != PolicyPriorModelSchemaVersion || model.ExampleCount != 1 || len(model.Moves) != 1 {
		t.Fatalf("bad model: %+v", model)
	}
	resp, err := (LocalPolicyPriorProvider{Model: model, Path: "local.json"}).CompleteJSON(context.Background(), providers.CompletionRequest{
		Model: "local.json",
		Metadata: map[string]string{
			"fen":         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			"legal_moves": "e2e4,d2d4",
		},
	})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	var out strategy.DecisionOutput
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil {
		t.Fatalf("provider returned invalid JSON: %v", err)
	}
	if out.CandidateMoves[0].UCI != "e2e4" || resp.Provider != "policy_prior" {
		t.Fatalf("unexpected response: provider=%+v out=%+v", resp, out)
	}
}

func TestPolicyPriorProviderFallsBackToLegalMove(t *testing.T) {
	model := PolicyPriorModel{
		SchemaVersion: PolicyPriorModelSchemaVersion,
		Moves: map[string]PolicyPrior{
			"known": {UCI: "e2e4", Confidence: 0.9},
		},
	}
	resp, err := (LocalPolicyPriorProvider{Model: model}).CompleteJSON(context.Background(), providers.CompletionRequest{
		Metadata: map[string]string{
			"fen":         "unknown",
			"legal_moves": "g1f3,b1c3",
		},
	})
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if !strings.Contains(resp.Text, "b1c3") {
		t.Fatalf("expected sorted legal fallback, got %s", resp.Text)
	}
}
