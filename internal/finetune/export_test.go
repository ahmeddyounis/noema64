package finetune

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestExportDecisionsJSONL(t *testing.T) {
	mem := strategy.NewMemory("game", "white")
	dec := decision.MoveDecision{
		DecisionID:      "dec",
		GameID:          "game",
		Mode:            strategy.ModeHybrid,
		SelectedMove:    chesscore.LegalMove{UCI: "e2e4"},
		Explanation:     "Take the center.",
		PositionSummary: "Start position.",
		StrategyBefore:  mem,
		StrategyAfter:   mem,
		FENBefore:       "start",
		LegalMovesCount: 20,
	}
	jsonl, workflow, err := ExportDecisionsJSONL([]decision.MoveDecision{dec})
	if err != nil {
		t.Fatalf("export decisions: %v", err)
	}
	if workflow.SchemaVersion != WorkflowSchemaVersion || workflow.ExampleCount != 1 {
		t.Fatalf("bad workflow: %+v", workflow)
	}
	var example DatasetExample
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonl)), &example); err != nil {
		t.Fatalf("decode jsonl: %v\n%s", err, jsonl)
	}
	if example.SchemaVersion != DatasetExampleSchemaVersion || example.Output.SelectedMove != "e2e4" || len(example.Messages) != 2 {
		t.Fatalf("bad example: %+v", example)
	}
}

func TestExportTraceJSONL(t *testing.T) {
	mem := strategy.NewMemory("game", "white")
	record := map[string]any{
		"trace": decision.MoveDecision{
			DecisionID:     "dec",
			GameID:         "game",
			SelectedMove:   chesscore.LegalMove{UCI: "g1f3"},
			StrategyBefore: mem,
			StrategyAfter:  mem,
		},
	}
	line, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	jsonl, workflow, err := ExportTraceJSONL(string(line) + "\n")
	if err != nil {
		t.Fatalf("export trace: %v", err)
	}
	if workflow.ExampleCount != 1 || !strings.Contains(jsonl, "g1f3") {
		t.Fatalf("unexpected trace export workflow=%+v jsonl=%s", workflow, jsonl)
	}
}
