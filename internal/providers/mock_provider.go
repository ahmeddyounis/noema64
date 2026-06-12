package providers

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

type MockProvider struct {
	Behavior string
}

func (p MockProvider) Name() string {
	if p.Behavior != "" {
		return "mock-" + p.Behavior
	}
	return "mock"
}

func (p MockProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsJSONMode:     true,
		SupportsCancellation: true,
		SupportsSeed:         true,
		MaxContextTokens:     32000,
		RecommendedMaxOutput: 1600,
	}
}

func (p MockProvider) HealthCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (p MockProvider) CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if p.Behavior == "invalid_json" {
		return &CompletionResponse{Text: "this is not json", Provider: p.Name(), Model: req.Model, Latency: time.Since(start)}, nil
	}
	if p.Behavior == "empty" {
		return &CompletionResponse{Text: "", Provider: p.Name(), Model: req.Model, Latency: time.Since(start)}, nil
	}
	if p.Behavior == "illegal" {
		return p.response(req, []string{"a1a8"}), nil
	}
	legal := strings.Split(req.Metadata["legal_moves"], ",")
	candidates := chooseMockMoves(legal, req.Metadata["max_candidates"])
	return p.response(req, candidates), nil
}

func (p MockProvider) response(req CompletionRequest, moves []string) *CompletionResponse {
	start := time.Now()
	candidates := make([]strategy.CandidateMove, 0, len(moves))
	for i, mv := range moves {
		conf := 0.80 - float64(i)*0.08
		candidates = append(candidates, strategy.CandidateMove{
			UCI:           mv,
			Purpose:       "Mock strategy chooses a legal, development-oriented move.",
			ExpectedReply: "Opponent continues with a legal reply.",
			Risk:          "No verifier-specific risk in mock mode.",
			LLMConfidence: conf,
		})
	}
	out := strategy.DecisionOutput{
		SchemaVersion:      strategy.DecisionSchemaVersion,
		PreviousPlanStatus: "continue",
		PositionSummary:    "Mock provider keeps play legal while preserving an inspectable strategy trace.",
		StrategyUpdate: strategy.StrategyUpdate{
			PlanSummary:        "Develop pieces, keep the king safe, and improve central control.",
			Phase:              "opening",
			MainTargets:        []string{"center", "king safety"},
			PieceImprovement:   []string{"activate undeveloped minor pieces"},
			PawnBreaks:         []string{"central pawn break when prepared"},
			OpponentPlanGuess:  "Opponent is expected to contest central squares.",
			Commitments:        []string{"Do not play illegal moves.", "Prefer safe development before speculative attacks."},
			RefutationTriggers: []string{"Opponent creates a direct tactic against the king."},
			TacticalWarnings:   []string{"Recheck forcing captures before committing."},
			Confidence:         0.66,
			LastUpdateSummary:  "Mock memory updated with a stable development plan.",
		},
		CandidateMoves: candidates,
	}
	b, _ := json.Marshal(out)
	return &CompletionResponse{
		Text:         string(b),
		Provider:     p.Name(),
		Model:        req.Model,
		Latency:      time.Since(start),
		RawAvailable: false,
	}
}

func chooseMockMoves(legal []string, _ string) []string {
	preferred := []string{"g1f3", "b1c3", "g8f6", "b8c6", "e2e4", "d2d4", "e7e5", "d7d5", "f1c4", "f8c5"}
	legalSet := map[string]struct{}{}
	for _, mv := range legal {
		mv = strings.TrimSpace(mv)
		if mv != "" {
			legalSet[mv] = struct{}{}
		}
	}
	out := make([]string, 0, 5)
	for _, mv := range preferred {
		if _, ok := legalSet[mv]; ok {
			out = append(out, mv)
		}
		if len(out) == 5 {
			return out
		}
	}
	for _, mv := range legal {
		mv = strings.TrimSpace(mv)
		if mv == "" {
			continue
		}
		out = append(out, mv)
		if len(out) == 5 {
			break
		}
	}
	return out
}
