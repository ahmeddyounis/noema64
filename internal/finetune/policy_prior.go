package finetune

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const PolicyPriorModelSchemaVersion = "policy-prior-model.v1"

type PolicyPriorModel struct {
	SchemaVersion string                 `json:"schema_version"`
	TrainedAt     string                 `json:"trained_at"`
	ExampleCount  int                    `json:"example_count"`
	Moves         map[string]PolicyPrior `json:"moves"`
}

type PolicyPrior struct {
	UCI          string  `json:"uci"`
	Explanation  string  `json:"explanation"`
	Position     string  `json:"position"`
	Count        int     `json:"count"`
	Confidence   float64 `json:"confidence"`
	LastSourceID string  `json:"last_source_id,omitempty"`
}

type LocalPolicyPriorProvider struct {
	Model PolicyPriorModel
	Path  string
}

func TrainPolicyPriorJSONL(datasetJSONL string) (PolicyPriorModel, error) {
	model := PolicyPriorModel{
		SchemaVersion: PolicyPriorModelSchemaVersion,
		TrainedAt:     time.Now().UTC().Format(time.RFC3339),
		Moves:         map[string]PolicyPrior{},
	}
	counts := map[string]map[string]int{}
	examples := map[string]DatasetExample{}
	scanner := bufio.NewScanner(strings.NewReader(datasetJSONL))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var example DatasetExample
		if err := json.Unmarshal([]byte(line), &example); err != nil {
			return PolicyPriorModel{}, err
		}
		fen := fenFromExample(example)
		move := strings.TrimSpace(example.Output.SelectedMove)
		if fen == "" || move == "" {
			continue
		}
		if counts[fen] == nil {
			counts[fen] = map[string]int{}
		}
		counts[fen][move]++
		examples[fen+" "+move] = example
		model.ExampleCount++
	}
	if err := scanner.Err(); err != nil {
		return PolicyPriorModel{}, err
	}
	for fen, byMove := range counts {
		type item struct {
			move  string
			count int
		}
		items := make([]item, 0, len(byMove))
		total := 0
		for move, count := range byMove {
			items = append(items, item{move: move, count: count})
			total += count
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].count == items[j].count {
				return items[i].move < items[j].move
			}
			return items[i].count > items[j].count
		})
		best := items[0]
		example := examples[fen+" "+best.move]
		model.Moves[fen] = PolicyPrior{
			UCI:          best.move,
			Explanation:  firstNonEmpty(example.Output.Explanation, "Local policy-prior selected the most frequent reviewed move for this exact position."),
			Position:     firstNonEmpty(example.Output.PositionSummary, "Exact position match from local policy-prior training data."),
			Count:        best.count,
			Confidence:   float64(best.count) / float64(total),
			LastSourceID: firstNonEmpty(example.DecisionID, example.GameID),
		}
	}
	if len(model.Moves) == 0 {
		return PolicyPriorModel{}, fmt.Errorf("dataset did not contain trainable FEN/move examples")
	}
	return model, nil
}

func SavePolicyPriorModel(path string, model PolicyPriorModel) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("policy-prior model path is required")
	}
	if model.SchemaVersion == "" {
		model.SchemaVersion = PolicyPriorModelSchemaVersion
	}
	b, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func LoadPolicyPriorModel(path string) (PolicyPriorModel, error) {
	b, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return PolicyPriorModel{}, err
	}
	var model PolicyPriorModel
	if err := json.Unmarshal(b, &model); err != nil {
		return PolicyPriorModel{}, err
	}
	if model.SchemaVersion != PolicyPriorModelSchemaVersion {
		return PolicyPriorModel{}, fmt.Errorf("unsupported policy-prior model schema_version %q", model.SchemaVersion)
	}
	if len(model.Moves) == 0 {
		return PolicyPriorModel{}, fmt.Errorf("policy-prior model has no moves")
	}
	return model, nil
}

func (p LocalPolicyPriorProvider) Name() string {
	return "policy_prior"
}

func (p LocalPolicyPriorProvider) Capabilities() providers.Capabilities {
	return providers.Capabilities{
		SupportsJSONMode:     true,
		SupportsCancellation: true,
		SupportsSeed:         true,
		MaxContextTokens:     8192,
		RecommendedMaxOutput: 800,
	}
}

func (p LocalPolicyPriorProvider) HealthCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if p.Model.SchemaVersion != PolicyPriorModelSchemaVersion || len(p.Model.Moves) == 0 {
		return fmt.Errorf("policy-prior model is not loaded")
	}
	return nil
}

func (p LocalPolicyPriorProvider) CompleteJSON(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	start := time.Now()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	fen := strings.TrimSpace(req.Metadata["fen"])
	legal := legalMoveSet(req.Metadata["legal_moves"])
	prior, ok := p.Model.Moves[fen]
	if !ok || !legal[prior.UCI] {
		prior = firstLegalPolicyPrior(legal)
		if prior.UCI == "" {
			return nil, fmt.Errorf("policy-prior provider had no legal fallback move")
		}
	}
	out := strategy.DecisionOutput{
		SchemaVersion:      strategy.DecisionSchemaVersion,
		PreviousPlanStatus: "continue",
		PositionSummary:    firstNonEmpty(prior.Position, "Local policy-prior exact-match candidate."),
		StrategyUpdate: strategy.StrategyUpdate{
			PlanSummary:        "Use a reviewed local policy-prior move while preserving legality and verifier constraints.",
			Phase:              "unknown",
			MainTargets:        []string{"local policy prior", "legal continuation"},
			Commitments:        []string{"Only emit moves present in LEGAL_MOVES."},
			RefutationTriggers: []string{"Verifier rejects the prior move.", "No exact policy-prior match exists."},
			Confidence:         prior.Confidence,
			LastUpdateSummary:  "Local policy-prior produced a candidate from the saved model.",
		},
		CandidateMoves: []strategy.CandidateMove{{
			UCI:           prior.UCI,
			Purpose:       firstNonEmpty(prior.Explanation, "Local policy-prior selected a legal candidate."),
			Risk:          "This is a distilled prior; verifier and search should still arbitrate tactical safety.",
			LLMConfidence: clamp(prior.Confidence, 0.35, 0.95),
		}},
		Metadata: map[string]string{
			"policy_prior_schema": p.Model.SchemaVersion,
			"policy_prior_path":   p.Path,
			"policy_prior_count":  fmt.Sprintf("%d", prior.Count),
		},
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &providers.CompletionResponse{
		Text:         string(b),
		Provider:     p.Name(),
		Model:        firstNonEmpty(req.Model, p.Path, "local-policy-prior"),
		Latency:      time.Since(start),
		RawAvailable: false,
		Metadata:     out.Metadata,
	}, nil
}

func fenFromExample(example DatasetExample) string {
	if fen := strings.TrimSpace(example.Metadata["fen"]); fen != "" {
		return fen
	}
	for _, msg := range example.Messages {
		for _, line := range strings.Split(msg.Content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "FEN:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "FEN:"))
			}
		}
	}
	return ""
}

func legalMoveSet(csv string) map[string]bool {
	out := map[string]bool{}
	for _, move := range strings.Split(csv, ",") {
		move = strings.TrimSpace(move)
		if move != "" {
			out[move] = true
		}
	}
	return out
}

func firstLegalPolicyPrior(legal map[string]bool) PolicyPrior {
	moves := make([]string, 0, len(legal))
	for move := range legal {
		moves = append(moves, move)
	}
	sort.Strings(moves)
	if len(moves) == 0 {
		return PolicyPrior{}
	}
	return PolicyPrior{
		UCI:         moves[0],
		Explanation: "Local policy-prior had no exact match; selected the first legal fallback for verifier arbitration.",
		Position:    "No exact local policy-prior match.",
		Confidence:  0.35,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
