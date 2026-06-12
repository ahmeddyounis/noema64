package decision

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

func TestHybridSearchScoresMaterialWin(t *testing.T) {
	game := searchTestGame(t)
	candidates := []strategy.CandidateMove{
		searchTestCandidate(t, game, "e1e2", 0.7),
		searchTestCandidate(t, game, "g3h4", 0.7),
	}

	used, name := applySearchScores(context.Background(), game, candidates, strategy.ModeHybrid)
	if !used {
		t.Fatal("expected hybrid search scores to be applied")
	}
	if name != deterministicSearchName {
		t.Fatalf("search name = %q, want %q", name, deterministicSearchName)
	}
	if candidates[1].SearchScore <= candidates[0].SearchScore {
		t.Fatalf("queen capture search score = %.2f, quiet move = %.2f", candidates[1].SearchScore, candidates[0].SearchScore)
	}

	scoreCandidates(candidates, strategy.StrategyMemory{}, strategy.ModeHybrid)
	if candidates[0].UCI != "g3h4" {
		t.Fatalf("top hybrid candidate = %s, want g3h4", candidates[0].UCI)
	}
}

func TestPureScoringIgnoresSearchScore(t *testing.T) {
	candidates := []strategy.CandidateMove{
		{UCI: "a2a3", Purpose: "high confidence", LLMConfidence: 0.9, SearchScore: -1},
		{UCI: "a2a4", Purpose: "low confidence", LLMConfidence: 0.1, SearchScore: 1},
	}

	scoreCandidates(candidates, strategy.StrategyMemory{}, strategy.ModePure)
	if candidates[0].UCI != "a2a3" {
		t.Fatalf("pure top candidate = %s, want a2a3", candidates[0].UCI)
	}
}

func TestChooseMoveDisclosesHybridSearchAssistance(t *testing.T) {
	game := searchTestGame(t)
	provider := scriptedProvider{
		moves: []strategy.CandidateMove{
			{UCI: "e1e2", Purpose: "Keep the king flexible.", LLMConfidence: 0.7},
			{UCI: "g3h4", Purpose: "Win the loose queen.", LLMConfidence: 0.7},
		},
	}

	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        strategy.NewMemory(game.ID(), "white"),
		Mode:          strategy.ModeHybrid,
		Provider:      provider,
		Verifier:      verifier.LegalOnlyVerifier{},
		Model:         "scripted",
		MaxCandidates: 2,
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("ChooseMove error = %v", err)
	}
	if dec.SelectedMove.UCI != "g3h4" {
		t.Fatalf("selected move = %s, want g3h4", dec.SelectedMove.UCI)
	}
	if !dec.Assistance.SearchUsed || dec.Assistance.SearchName != deterministicSearchName {
		t.Fatalf("search assistance = %+v, want disclosed deterministic search", dec.Assistance)
	}
	if dec.Timing.SearchMS < 0 {
		t.Fatalf("search timing = %d, want non-negative", dec.Timing.SearchMS)
	}
	if len(dec.CandidateMoves) == 0 || dec.CandidateMoves[0].SearchScore <= 0 {
		t.Fatalf("top candidate search score was not recorded: %+v", dec.CandidateMoves)
	}
}

func TestChooseMoveRawProviderLoggingIsOptIn(t *testing.T) {
	game := searchTestGame(t)
	provider := scriptedProvider{
		moves: []strategy.CandidateMove{{UCI: "g3h4", Purpose: "Win the loose queen.", LLMConfidence: 0.7}},
	}

	defaultTrace, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        strategy.NewMemory(game.ID(), "white"),
		Mode:          strategy.ModePure,
		Provider:      provider,
		Verifier:      verifier.LegalOnlyVerifier{},
		Model:         "scripted",
		MaxCandidates: 1,
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("default ChooseMove error = %v", err)
	}
	if defaultTrace.Provider.RawPrompt != nil || defaultTrace.Provider.RawResponse != "" {
		t.Fatalf("raw provider data logged by default: %+v", defaultTrace.Provider)
	}

	rawTrace, err := ChooseMove(context.Background(), Request{
		Game:           game,
		Memory:         strategy.NewMemory(game.ID(), "white"),
		Mode:           strategy.ModePure,
		Provider:       provider,
		Verifier:       verifier.LegalOnlyVerifier{},
		Model:          "scripted",
		MaxCandidates:  1,
		Timeout:        time.Second,
		LogRawPrompts:  true,
		LogRawResponse: true,
	})
	if err != nil {
		t.Fatalf("raw logging ChooseMove error = %v", err)
	}
	if rawTrace.Provider.RawPrompt == nil {
		t.Fatal("raw prompt was not logged when enabled")
	}
	if !strings.Contains(rawTrace.Provider.RawPrompt.User, "g3h4") {
		t.Fatalf("raw prompt does not include legal/candidate context: %s", rawTrace.Provider.RawPrompt.User)
	}
	if !strings.Contains(rawTrace.Provider.RawResponse, "candidate_moves") {
		t.Fatalf("raw response was not logged: %q", rawTrace.Provider.RawResponse)
	}
}

func searchTestGame(t *testing.T) *chesscore.Game {
	t.Helper()
	game, err := chesscore.FromFEN("4k3/8/8/8/7q/6P1/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("test FEN: %v", err)
	}
	return game
}

func searchTestCandidate(t *testing.T, game *chesscore.Game, uci string, confidence float64) strategy.CandidateMove {
	t.Helper()
	mv, ok := game.NormalizeMove(uci)
	if !ok {
		t.Fatalf("move %s is not legal in test position", uci)
	}
	return strategy.CandidateMove{
		UCI:           uci,
		Purpose:       "test candidate",
		LLMConfidence: confidence,
		VerifierScore: strategy.VerifierScore{Status: "accepted", Reason: "test"},
		LegalMove:     mv,
	}
}

type scriptedProvider struct {
	moves []strategy.CandidateMove
}

func (p scriptedProvider) Name() string {
	return "scripted"
}

func (p scriptedProvider) Capabilities() providers.Capabilities {
	return providers.Capabilities{SupportsJSONMode: true, SupportsCancellation: true}
}

func (p scriptedProvider) HealthCheck(ctx context.Context) error {
	return ctx.Err()
}

func (p scriptedProvider) CompleteJSON(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := strategy.DecisionOutput{
		SchemaVersion:      strategy.DecisionSchemaVersion,
		PreviousPlanStatus: "continue",
		PositionSummary:    "White can win a loose queen.",
		StrategyUpdate: strategy.StrategyUpdate{
			PlanSummary:       "Win material when tactics permit.",
			Phase:             "endgame",
			OpponentPlanGuess: "Black hopes to keep queen activity.",
			Confidence:        0.7,
			LastUpdateSummary: "Search-confirmed material win.",
		},
		CandidateMoves: p.moves,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &providers.CompletionResponse{
		Text:         string(b),
		Provider:     p.Name(),
		Model:        req.Model,
		RawAvailable: true,
	}, nil
}
