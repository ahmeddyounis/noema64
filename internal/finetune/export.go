package finetune

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const (
	DatasetExampleSchemaVersion = "fine-tune-example.v1"
	WorkflowSchemaVersion       = "fine-tune-workflow.v1"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DatasetExample struct {
	SchemaVersion string            `json:"schema_version"`
	Source        string            `json:"source"`
	GameID        string            `json:"game_id,omitempty"`
	DecisionID    string            `json:"decision_id,omitempty"`
	Messages      []Message         `json:"messages"`
	Output        FineTuneOutput    `json:"output"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type FineTuneOutput struct {
	SelectedMove       string                  `json:"selected_move"`
	Explanation        string                  `json:"explanation"`
	PositionSummary    string                  `json:"position_summary"`
	StrategyUpdate     strategy.StrategyMemory `json:"strategy_after"`
	CandidateMoveCount int                     `json:"candidate_move_count"`
}

type WorkflowSpec struct {
	SchemaVersion string   `json:"schema_version"`
	GeneratedAt   string   `json:"generated_at"`
	ExampleCount  int      `json:"example_count"`
	IntendedUse   string   `json:"intended_use"`
	Format        string   `json:"format"`
	SafetyNotes   []string `json:"safety_notes"`
	NextSteps     []string `json:"next_steps"`
}

func ExportDecisionsJSONL(decisions []decision.MoveDecision) (string, WorkflowSpec, error) {
	var b bytes.Buffer
	count := 0
	for _, dec := range decisions {
		example := ExampleFromDecision(dec)
		line, err := json.Marshal(example)
		if err != nil {
			return "", WorkflowSpec{}, err
		}
		b.Write(line)
		b.WriteByte('\n')
		count++
	}
	return b.String(), NewWorkflowSpec(count), nil
}

func ExampleFromDecision(dec decision.MoveDecision) DatasetExample {
	system := "You are Noema64, a chess engine that must choose legal moves and maintain auditable strategy memory."
	user := strings.Join([]string{
		"FEN: " + dec.FENBefore,
		fmt.Sprintf("Mode: %s", dec.Mode),
		fmt.Sprintf("Legal moves count: %d", dec.LegalMovesCount),
		"Position summary: " + dec.PositionSummary,
		"Strategy before: " + compactJSON(dec.StrategyBefore),
		"Candidate moves: " + compactJSON(dec.CandidateMoves),
	}, "\n")
	return DatasetExample{
		SchemaVersion: DatasetExampleSchemaVersion,
		Source:        "decision_trace",
		GameID:        dec.GameID,
		DecisionID:    dec.DecisionID,
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Output: FineTuneOutput{
			SelectedMove:       dec.SelectedMove.UCI,
			Explanation:        dec.Explanation,
			PositionSummary:    dec.PositionSummary,
			StrategyUpdate:     dec.StrategyAfter,
			CandidateMoveCount: len(dec.CandidateMoves),
		},
		Metadata: map[string]string{
			"provider":       dec.Provider.Name,
			"model":          dec.Provider.Model,
			"fallback_used":  fmt.Sprintf("%t", dec.FallbackUsed),
			"analysis_only":  fmt.Sprintf("%t", dec.AnalysisOnly),
			"prompt_version": dec.Provider.PromptVersion,
		},
	}
}

func ExportTraceJSONL(traceJSONL string) (string, WorkflowSpec, error) {
	decisions := []decision.MoveDecision{}
	scanner := bufio.NewScanner(strings.NewReader(traceJSONL))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record struct {
			Trace decision.MoveDecision `json:"trace"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return "", WorkflowSpec{}, err
		}
		if record.Trace.DecisionID == "" {
			continue
		}
		decisions = append(decisions, record.Trace)
	}
	if err := scanner.Err(); err != nil {
		return "", WorkflowSpec{}, err
	}
	return ExportDecisionsJSONL(decisions)
}

func NewWorkflowSpec(exampleCount int) WorkflowSpec {
	return WorkflowSpec{
		SchemaVersion: WorkflowSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ExampleCount:  exampleCount,
		IntendedUse:   "Fine-tune or distill a small local policy-prior model for move explanation and candidate ordering.",
		Format:        "jsonl: each line contains messages plus the selected move, explanation, and strategy memory output.",
		SafetyNotes: []string{
			"Dataset export is local-only and does not upload examples.",
			"Raw prompts and raw provider responses are excluded unless separately exported through debug traces.",
			"Examples should be reviewed for provider terms and private game data before external training.",
		},
		NextSteps: []string{
			"Collect enough reviewed decisions for the target opening/phase distribution.",
			"Split examples into train and validation sets outside Noema64.",
			"Evaluate the resulting model with position suites before enabling it in play.",
		},
	}
}

func compactJSON(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(b)
}
