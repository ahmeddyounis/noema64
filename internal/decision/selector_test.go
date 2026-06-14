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

	scoreCandidates(candidates, strategy.StrategyMemory{}, strategy.ModeHybrid, strategy.ProfileForPersonality(strategy.PersonalityBalanced))
	if candidates[0].UCI != "g3h4" {
		t.Fatalf("top hybrid candidate = %s, want g3h4", candidates[0].UCI)
	}
}

func TestBlunderguardSearchScoresMaterialWin(t *testing.T) {
	game := searchTestGame(t)
	candidates := []strategy.CandidateMove{
		searchTestCandidate(t, game, "e1e2", 0.99),
		searchTestCandidate(t, game, "g3h4", 0.30),
	}

	used, name := applySearchScores(context.Background(), game, candidates, strategy.ModeBlunderguard)
	if !used {
		t.Fatal("expected blunderguard search scores to be applied")
	}
	if name != deterministicSearchName {
		t.Fatalf("search name = %q, want %q", name, deterministicSearchName)
	}

	scoreCandidates(candidates, strategy.StrategyMemory{}, strategy.ModeBlunderguard, strategy.ProfileForPersonality(strategy.PersonalityBalanced))
	if candidates[0].UCI != "g3h4" {
		t.Fatalf("top blunderguard candidate = %s, want queen capture g3h4; candidates=%+v", candidates[0].UCI, candidates)
	}
}

func TestChooseMoveBlunderguardAddsTacticalGuardrailCandidate(t *testing.T) {
	game := searchTestGame(t)
	provider := scriptedProvider{
		moves: []strategy.CandidateMove{
			{UCI: "e1e2", Purpose: "Keep the king flexible.", LLMConfidence: 0.99},
		},
	}

	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        strategy.NewMemory(game.ID(), "white"),
		Mode:          strategy.ModeBlunderguard,
		Provider:      provider,
		Verifier:      verifier.LegalOnlyVerifier{},
		Model:         "scripted",
		MaxCandidates: 1,
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("ChooseMove error = %v", err)
	}
	if dec.SelectedMove.UCI != "g3h4" {
		t.Fatalf("selected move = %s, want guardrail queen capture g3h4; candidates=%+v", dec.SelectedMove.UCI, dec.CandidateMoves)
	}
	if !dec.Assistance.SearchUsed || dec.Assistance.SearchName != deterministicSearchName {
		t.Fatalf("search assistance = %+v, want blunderguard search", dec.Assistance)
	}
	foundGuardrail := false
	for _, candidate := range dec.CandidateMoves {
		if candidate.UCI == "g3h4" && candidate.RepairMethod == "engine_guardrail" {
			foundGuardrail = true
			break
		}
	}
	if !foundGuardrail {
		t.Fatalf("guardrail candidate was not added: %+v", dec.CandidateMoves)
	}
}

func TestCurrentModeUsesSearchAndResetsStrategyMemory(t *testing.T) {
	game := searchTestGame(t)
	staleMemory := strategy.NewMemory(game.ID(), "white")
	staleMemory.Plan.Summary = "Preserve an old long-term king walk."
	staleMemory.Targets.Squares = []string{"e2"}
	staleMemory.Commitments = []string{"Keep the king flexible."}
	provider := scriptedProvider{
		moves: []strategy.CandidateMove{
			{UCI: "e1e2", Purpose: "Follow the old king-walk plan.", LLMConfidence: 0.95},
			{UCI: "g3h4", Purpose: "Win the loose queen immediately.", LLMConfidence: 0.55},
		},
	}

	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        staleMemory,
		Mode:          strategy.ModeCurrent,
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
		t.Fatalf("selected move = %s, want immediate queen capture g3h4; candidates=%+v", dec.SelectedMove.UCI, dec.CandidateMoves)
	}
	if !dec.Assistance.SearchUsed || dec.Assistance.SearchName != deterministicSearchName {
		t.Fatalf("search assistance = %+v, want current-position search", dec.Assistance)
	}
	if dec.StrategyBefore.Plan.Summary == staleMemory.Plan.Summary {
		t.Fatalf("current mode reused stale strategy memory: %+v", dec.StrategyBefore.Plan)
	}
	for _, candidate := range dec.CandidateMoves {
		if candidate.PlanAlignmentScore != 0 {
			t.Fatalf("current mode candidate kept plan alignment: %+v", candidate)
		}
	}
}

func TestChooseMovePassesCurrentGameContextToProvider(t *testing.T) {
	game := chesscore.NewGame()
	if _, err := game.ApplyUCI("e2e4"); err != nil {
		t.Fatalf("apply opening move: %v", err)
	}
	game.AnnotateLastMove("Plan: stale comment should not reach the provider prompt")
	provider := &capturingProvider{}
	clock := map[string]int64{"white_ms": 298000, "black_ms": 300000, "increment_ms": 1000}

	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Variant:       chesscore.StandardStart(game.InitialFEN()),
		Clock:         clock,
		Memory:        strategy.NewMemory(game.ID(), "black"),
		Mode:          strategy.ModeCurrent,
		Provider:      provider,
		Verifier:      verifier.LegalOnlyVerifier{},
		Model:         "capture",
		MaxCandidates: 2,
		Timeout:       time.Second,
		LogRawPrompts: true,
	})
	if err != nil {
		t.Fatalf("ChooseMove error = %v", err)
	}
	if provider.request.Metadata["fen"] != game.FEN() {
		t.Fatalf("metadata fen = %q, want current %q", provider.request.Metadata["fen"], game.FEN())
	}
	wantMetadata := map[string]string{
		"side_to_move":       "black",
		"ply":                "1",
		"move_number":        "1",
		"last_opponent_move": "e2e4",
		"mode":               "current",
		"variant":            "standard",
	}
	for key, want := range wantMetadata {
		if got := provider.request.Metadata[key]; got != want {
			t.Fatalf("metadata[%s] = %q, want %q", key, got, want)
		}
	}
	if dec.FENBefore != game.FEN() || dec.LegalMovesCount != len(game.LegalMoves()) {
		t.Fatalf("decision current context mismatch: fen=%q legal=%d", dec.FENBefore, dec.LegalMovesCount)
	}
	userPrompt := provider.request.User
	for _, want := range []string{
		game.FEN(),
		`"side_to_move": "black"`,
		`"ply": 1`,
		`"variant": "standard"`,
		`"white_ms": 298000`,
		`"black_ms": 300000`,
		`"increment_ms": 1000`,
		"Last opponent move: BEGIN_UNTRUSTED_CHESS_TEXT\ne2e4",
		"ENGINE_MODE\ncurrent",
	} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("provider prompt missing %q:\n%s", want, userPrompt)
		}
	}
	if strings.Contains(userPrompt, "Plan: stale") {
		t.Fatalf("provider prompt leaked PGN comment:\n%s", userPrompt)
	}
}

func TestChooseMoveGroundsMaterialSummary(t *testing.T) {
	game := chesscore.NewGame()
	provider := scriptedProvider{
		positionSummary: "Black is materially ahead by a piece.",
		moves:           []strategy.CandidateMove{{UCI: "g1f3", Purpose: "Develop the knight.", LLMConfidence: 0.7}},
	}

	dec, err := ChooseMove(context.Background(), Request{
		Game:          game,
		Memory:        strategy.NewMemory(game.ID(), "white"),
		Mode:          strategy.ModeBlunderguard,
		Provider:      provider,
		Verifier:      verifier.LegalOnlyVerifier{},
		Model:         "scripted",
		MaxCandidates: 1,
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("ChooseMove error = %v", err)
	}
	if dec.PositionSummary != "Material is equal." {
		t.Fatalf("position summary = %q, want deterministic equality summary", dec.PositionSummary)
	}
	if strings.Contains(dec.PositionSummary, "Black is materially ahead") {
		t.Fatalf("provider material hallucination leaked into summary: %q", dec.PositionSummary)
	}
}

func TestPureScoringIgnoresSearchScore(t *testing.T) {
	candidates := []strategy.CandidateMove{
		{UCI: "a2a3", Purpose: "high confidence", LLMConfidence: 0.9, SearchScore: -1},
		{UCI: "a2a4", Purpose: "low confidence", LLMConfidence: 0.1, SearchScore: 1},
	}

	scoreCandidates(candidates, strategy.StrategyMemory{}, strategy.ModePure, strategy.ProfileForPersonality(strategy.PersonalityBalanced))
	if candidates[0].UCI != "a2a3" {
		t.Fatalf("pure top candidate = %s, want a2a3", candidates[0].UCI)
	}
}

func TestPersonalityScoreInfluencesCloseCandidateRanking(t *testing.T) {
	game := searchTestGame(t)
	closeRace := func() []strategy.CandidateMove {
		return []strategy.CandidateMove{
			searchTestCandidate(t, game, "e1e2", 0.76),
			searchTestCandidate(t, game, "g3h4", 0.72),
		}
	}

	aggressive := closeRace()
	scoreCandidates(aggressive, strategy.StrategyMemory{}, strategy.ModePure, strategy.ProfileForPersonality(strategy.PersonalityAggressive))
	if aggressive[0].UCI != "g3h4" {
		t.Fatalf("aggressive top candidate = %s, want forcing capture g3h4; candidates=%+v", aggressive[0].UCI, aggressive)
	}
	if aggressive[0].PersonalityScore <= 0 {
		t.Fatalf("aggressive capture personality score = %.2f, want positive", aggressive[0].PersonalityScore)
	}

	positional := closeRace()
	scoreCandidates(positional, strategy.StrategyMemory{}, strategy.ModePure, strategy.ProfileForPersonality(strategy.PersonalityPositional))
	if positional[0].UCI != "e1e2" {
		t.Fatalf("positional top candidate = %s, want quiet move e1e2; candidates=%+v", positional[0].UCI, positional)
	}
	if positional[0].PersonalityScore <= 0 {
		t.Fatalf("positional quiet personality score = %.2f, want positive", positional[0].PersonalityScore)
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
	if rawTrace.Provider.ParsedDecision == nil || rawTrace.Provider.ParsedDecision.PositionSummary == "" {
		t.Fatalf("parsed provider decision was not retained: %+v", rawTrace.Provider)
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
	moves           []strategy.CandidateMove
	positionSummary string
}

type capturingProvider struct {
	request providers.CompletionRequest
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
	positionSummary := p.positionSummary
	if positionSummary == "" {
		positionSummary = "White can win a loose queen."
	}
	out := strategy.DecisionOutput{
		SchemaVersion:      strategy.DecisionSchemaVersion,
		PreviousPlanStatus: "continue",
		PositionSummary:    positionSummary,
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

func (p *capturingProvider) Name() string {
	return "capturing"
}

func (p *capturingProvider) Capabilities() providers.Capabilities {
	return providers.Capabilities{SupportsJSONMode: true, SupportsCancellation: true}
}

func (p *capturingProvider) HealthCheck(ctx context.Context) error {
	return ctx.Err()
}

func (p *capturingProvider) CompleteJSON(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	p.request = req
	legal := strings.Split(req.Metadata["legal_moves"], ",")
	move := strings.TrimSpace(legal[0])
	out := strategy.DecisionOutput{
		SchemaVersion:      strategy.DecisionSchemaVersion,
		PreviousPlanStatus: "new",
		PositionSummary:    "Captured current context.",
		StrategyUpdate: strategy.StrategyUpdate{
			PlanSummary:       "Use current context.",
			Phase:             "opening",
			OpponentPlanGuess: "Opponent develops.",
			Confidence:        0.7,
			LastUpdateSummary: "Context captured.",
		},
		CandidateMoves: []strategy.CandidateMove{{UCI: move, Purpose: "First legal move from captured metadata.", LLMConfidence: 0.7}},
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &providers.CompletionResponse{Text: string(b), Provider: p.Name(), Model: req.Model, RawAvailable: true}, nil
}
