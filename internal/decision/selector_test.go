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

func TestChooseMovePureModeBypassesConfiguredVerifier(t *testing.T) {
	game := searchTestGame(t)
	verifierCalls := 0
	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        strategy.NewMemory(game.ID(), "white"),
		Mode:          strategy.ModePure,
		Provider:      scriptedProvider{moves: []strategy.CandidateMove{{UCI: "g3h4", Purpose: "Win the loose queen.", LLMConfidence: 0.7}}},
		Verifier:      countingVerifier{calls: &verifierCalls},
		Model:         "scripted",
		MaxCandidates: 1,
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("ChooseMove error = %v", err)
	}
	if verifierCalls != 0 {
		t.Fatalf("pure mode called configured verifier %d times", verifierCalls)
	}
	if dec.VerifierTrace == nil || dec.VerifierTrace.Name != "legal_only" || dec.Assistance.VerifierUsed {
		t.Fatalf("pure mode verifier trace = %+v assistance = %+v", dec.VerifierTrace, dec.Assistance)
	}
}

func TestChooseMoveRecordsStagesAndProgressEvents(t *testing.T) {
	game := searchTestGame(t)
	provider := scriptedProvider{
		moves: []strategy.CandidateMove{{UCI: "g3h4", Purpose: "Win the loose queen.", LLMConfidence: 0.7}},
	}
	events := []ProgressEvent{}

	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        strategy.NewMemory(game.ID(), "white"),
		Mode:          strategy.ModeHybrid,
		Provider:      provider,
		Verifier:      verifier.LegalOnlyVerifier{},
		Model:         "scripted",
		MaxCandidates: 1,
		Timeout:       time.Second,
		Progress: func(event ProgressEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("ChooseMove error = %v", err)
	}

	wantStages := []string{
		"reading_position",
		"updating_strategy_memory",
		"asking_provider",
		"repairing_candidate_moves",
		"verifying_tactics",
		"scoring_candidates",
		"updating_strategy_memory_after_move",
	}
	if got := stageNames(dec.Stages); !sameStrings(got, wantStages) {
		t.Fatalf("stages = %v, want %v", got, wantStages)
	}
	if len(events) < len(wantStages)*2 {
		t.Fatalf("progress events = %d, want at least start/finish for %d stages", len(events), len(wantStages))
	}
	if events[0].EventName != DecisionStageEvent || events[0].Stage != "reading_position" || events[0].Status != "started" {
		t.Fatalf("first progress event = %+v", events[0])
	}
	last := events[len(events)-1]
	if last.Stage != "updating_strategy_memory_after_move" || last.Status != "completed" || last.GameID == "" {
		t.Fatalf("last progress event = %+v", last)
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

func stageNames(stages []StageTrace) []string {
	names := make([]string, 0, len(stages))
	for _, stage := range stages {
		names = append(names, stage.Name)
	}
	return names
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

type scriptedProvider struct {
	moves []strategy.CandidateMove
}

type countingVerifier struct {
	calls *int
}

func (v countingVerifier) Name() string {
	return "counting_verifier"
}

func (v countingVerifier) VerifyCandidates(ctx context.Context, req verifier.Request) (*verifier.Result, error) {
	*v.calls++
	return (verifier.StaticVerifier{Enabled: true}).VerifyCandidates(ctx, req)
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
