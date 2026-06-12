package decision

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

func ChooseMove(ctx context.Context, req Request) (*MoveDecision, error) {
	if req.Game == nil {
		return nil, fmt.Errorf("nil game")
	}
	legal := req.Game.LegalMoves()
	if len(legal) == 0 {
		return nil, fmt.Errorf("no legal moves")
	}
	if req.Timeout <= 0 {
		req.Timeout = 12 * time.Second
	}
	if req.MaxCandidates <= 0 {
		req.MaxCandidates = 5
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1600
	}
	if req.Model == "" {
		req.Model = "mock-balanced"
	}
	if req.Provider == nil {
		req.Provider = providers.MockProvider{}
	}
	if req.Verifier == nil {
		req.Verifier = verifier.StaticVerifier{}
	}

	start := time.Now()
	decisionID := "dec_" + uuid.NewString()
	snapshot := req.Game.Snapshot()
	memBefore := req.Memory
	stratReq := strategy.StrategyRequest{
		GameID:           snapshot.GameID,
		FEN:              snapshot.FEN,
		PGN:              snapshot.PGN,
		SideToMove:       snapshot.SideToMove,
		MoveNumber:       snapshot.Ply/2 + 1,
		LastOpponentMove: lastMove(snapshot.MoveHistory),
		LegalMoves:       legal,
		Features:         req.Game.Features(),
		PreviousMemory:   req.Memory,
		Mode:             req.Mode,
		Personality:      req.Personality,
	}
	system, user, err := strategy.BuildPrompt(stratReq)
	if err != nil {
		return fallbackDecision(req, decisionID, memBefore, "prompt_build_error", start, nil), nil
	}

	providerCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	providerStart := time.Now()
	resp, err := req.Provider.CompleteJSON(providerCtx, providers.CompletionRequest{
		Model:       req.Model,
		System:      system,
		User:        user,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Metadata: map[string]string{
			"legal_moves":    strategy.LegalMoveCSV(stratReq),
			"max_candidates": strconv.Itoa(req.MaxCandidates),
			"game_id":        snapshot.GameID,
		},
	})
	providerMS := time.Since(providerStart).Milliseconds()
	providerTrace := ProviderTrace{
		Name:          req.Provider.Name(),
		Model:         req.Model,
		PromptVersion: strategy.PromptVersion,
	}
	if req.LogRawPrompts {
		providerTrace.RawPrompt = &PromptTrace{System: system, User: user}
	}
	if err != nil {
		providerTrace.Error = err.Error()
		return fallbackDecision(req, decisionID, memBefore, "provider_error", start, &providerTrace), nil
	}
	providerTrace.RawAvailable = resp.RawAvailable
	providerTrace.Name = resp.Provider
	providerTrace.Model = resp.Model
	if req.LogRawResponse {
		providerTrace.RawResponse = resp.Text
	}

	parse := strategy.ParseDecision(resp.Text)
	providerTrace.ParseStatus = parse.Status
	if parse.Status != "ok" && parse.Status != "extracted_json" {
		providerTrace.Error = parse.Error
		return fallbackDecision(req, decisionID, memBefore, "provider_schema_invalid", start, &providerTrace), nil
	}

	candidates, repairs := strategy.NormalizeCandidates(req.Game, parse.Decision.CandidateMoves)
	if len(candidates) == 0 {
		return fallbackDecision(req, decisionID, memBefore, "no_legal_llm_candidates", start, &providerTrace), nil
	}
	if len(candidates) > req.MaxCandidates {
		candidates = candidates[:req.MaxCandidates]
	}

	verifierStart := time.Now()
	verifyResult, err := req.Verifier.VerifyCandidates(ctx, verifier.Request{
		Game:       req.Game.Clone(),
		FEN:        snapshot.FEN,
		Candidates: candidates,
		Mode:       req.Mode,
	})
	verifierMS := time.Since(verifierStart).Milliseconds()
	if err != nil {
		verifyResult = &verifier.Result{Enabled: req.Mode != strategy.ModePure, Used: false, Name: req.Verifier.Name(), Error: err.Error()}
	}
	applyVerifierScores(candidates, verifyResult)
	searchStart := time.Now()
	searchUsed, searchName := applySearchScores(ctx, req.Game, candidates, req.Mode)
	searchMS := time.Since(searchStart).Milliseconds()
	scoreCandidates(candidates, req.Memory, req.Mode)
	chosen, ok := selectCandidate(candidates, req.Mode)
	if !ok {
		return fallbackDecision(req, decisionID, memBefore, "arbiter_no_move", start, &providerTrace), nil
	}

	memAfter := strategy.MergeMemory(memBefore, parse.Decision.StrategyUpdate, snapshot.GameID, snapshot.SideToMove, snapshot.Ply+1, decisionID, chosen.UCI)
	diff := strategy.DiffMemory(memBefore, memAfter)
	totalMS := time.Since(start).Milliseconds()
	return &MoveDecision{
		SchemaVersion:      "decision-trace.v1",
		DecisionID:         decisionID,
		GameID:             snapshot.GameID,
		Ply:                snapshot.Ply + 1,
		Mode:               req.Mode,
		SelectedMove:       chosen.LegalMove,
		Explanation:        chosen.Purpose,
		PositionSummary:    parse.Decision.PositionSummary,
		PreviousPlanStatus: parse.Decision.PreviousPlanStatus,
		StrategyBefore:     memBefore,
		StrategyAfter:      memAfter,
		StrategyDiff:       diff,
		CandidateMoves:     candidates,
		RepairAttempts:     repairs,
		VerifierTrace:      verifyResult,
		FallbackUsed:       false,
		Provider:           providerTrace,
		Timing: Timing{
			TotalMS:    totalMS,
			ProviderMS: providerMS,
			VerifierMS: verifierMS,
			SearchMS:   searchMS,
			OtherMS:    remainingTimingMS(totalMS, providerMS, verifierMS, searchMS),
		},
		Assistance: AssistanceTrace{
			Mode:         req.Mode,
			VerifierUsed: verifyResult != nil && verifyResult.Used,
			VerifierName: verifierName(verifyResult),
			SearchUsed:   searchUsed,
			SearchName:   searchName,
		},
		FENBefore:       snapshot.FEN,
		LegalMovesCount: len(legal),
	}, nil
}

func fallbackDecision(req Request, decisionID string, mem strategy.StrategyMemory, reason string, start time.Time, providerTrace *ProviderTrace) *MoveDecision {
	if providerTrace == nil {
		providerTrace = &ProviderTrace{Name: req.Provider.Name(), Model: req.Model, PromptVersion: strategy.PromptVersion}
	}
	snapshot := req.Game.Snapshot()
	mv := chooseFallback(req.Game)
	update := strategy.StrategyUpdate{
		PlanSummary:        mem.Plan.Summary,
		Phase:              req.Game.Features().Phase,
		MainTargets:        []string{"legal continuation", "king safety"},
		OpponentPlanGuess:  mem.OpponentModel.LikelyPlan,
		Commitments:        append(mem.Commitments, "Fallback selected a legal move after provider/verifier failure."),
		TacticalWarnings:   []string{"Fallback move should be reviewed tactically."},
		Confidence:         math.Max(0.2, mem.Plan.Confidence-0.1),
		LastUpdateSummary:  "Fallback ladder preserved legal play after " + reason + ".",
		RefutationTriggers: []string{"Provider remains unavailable or malformed."},
	}
	memAfter := strategy.MergeMemory(mem, update, snapshot.GameID, snapshot.SideToMove, snapshot.Ply+1, decisionID, mv.UCI)
	diff := strategy.DiffMemory(mem, memAfter)
	candidate := strategy.CandidateMove{
		UCI:           mv.UCI,
		SAN:           mv.SAN,
		Purpose:       "Fallback ladder selected a deterministic legal move.",
		Risk:          "Chosen without provider confidence.",
		LLMConfidence: 0,
		FinalScore:    0,
		Rank:          1,
		LegalMove:     mv,
		VerifierScore: strategy.VerifierScore{Status: "not_checked", Reason: "Fallback selected before verifier ranking."},
	}
	return &MoveDecision{
		SchemaVersion:      "decision-trace.v1",
		DecisionID:         decisionID,
		GameID:             snapshot.GameID,
		Ply:                snapshot.Ply + 1,
		Mode:               req.Mode,
		SelectedMove:       mv,
		Explanation:        candidate.Purpose,
		PositionSummary:    "Fallback legal move selected.",
		PreviousPlanStatus: "modify",
		StrategyBefore:     mem,
		StrategyAfter:      memAfter,
		StrategyDiff:       diff,
		CandidateMoves:     []strategy.CandidateMove{candidate},
		VerifierTrace:      &verifier.Result{Enabled: req.Mode != strategy.ModePure, Used: false, Name: "fallback"},
		FallbackUsed:       true,
		FallbackReason:     reason,
		Provider:           *providerTrace,
		Timing:             Timing{TotalMS: time.Since(start).Milliseconds()},
		Assistance:         AssistanceTrace{Mode: req.Mode, VerifierUsed: false},
		FENBefore:          snapshot.FEN,
		LegalMovesCount:    len(snapshot.LegalMoves),
	}
}

func chooseFallback(game *chesscore.Game) chesscore.LegalMove {
	legal := game.LegalMoves()
	sort.SliceStable(legal, func(i, j int) bool {
		return fallbackScore(legal[i]) > fallbackScore(legal[j])
	})
	return legal[0]
}

func fallbackScore(mv chesscore.LegalMove) int {
	score := 0
	if mv.Check {
		score += 100
	}
	if mv.Capture {
		score += 50
	}
	switch mv.To {
	case "d4", "e4", "d5", "e5", "c4", "f4", "c5", "f5":
		score += 20
	}
	switch mv.From {
	case "g1", "b1", "g8", "b8":
		score += 15
	}
	return score
}

func applyVerifierScores(candidates []strategy.CandidateMove, result *verifier.Result) {
	if result == nil {
		return
	}
	byMove := map[string]strategy.VerifierScore{}
	for _, item := range result.Candidates {
		byMove[item.UCI] = item.Score
	}
	for i := range candidates {
		if score, ok := byMove[candidates[i].UCI]; ok {
			candidates[i].VerifierScore = score
		}
	}
}

func scoreCandidates(candidates []strategy.CandidateMove, mem strategy.StrategyMemory, mode strategy.EngineMode) {
	for i := range candidates {
		plan := planAlignment(candidates[i], mem)
		candidates[i].PlanAlignmentScore = plan
		tactical := 0.5
		switch candidates[i].VerifierScore.Status {
		case "accepted":
			tactical = 1
		case "warning":
			tactical = 0.35
		case "rejected":
			tactical = -1
		}
		llm := candidates[i].LLMConfidence
		switch mode {
		case strategy.ModeHybrid:
			candidates[i].FinalScore = 0.20*llm + 0.20*plan + 0.20*tactical + 0.35*candidates[i].SearchScore
		case strategy.ModeBlunderguard:
			candidates[i].FinalScore = 0.35*llm + 0.25*plan + 0.30*tactical
		default:
			candidates[i].FinalScore = 0.55*llm + 0.35*plan
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].FinalScore > candidates[j].FinalScore
	})
	for i := range candidates {
		candidates[i].Rank = i + 1
	}
}

func selectCandidate(candidates []strategy.CandidateMove, mode strategy.EngineMode) (strategy.CandidateMove, bool) {
	for _, candidate := range candidates {
		if (mode == strategy.ModeBlunderguard || mode == strategy.ModeHybrid) && candidate.VerifierScore.Status == "rejected" {
			continue
		}
		return candidate, true
	}
	return strategy.CandidateMove{}, false
}

func planAlignment(candidate strategy.CandidateMove, mem strategy.StrategyMemory) float64 {
	score := 0.2
	text := candidate.Purpose + " " + candidate.SAN + " " + candidate.UCI
	for _, target := range mem.Targets.Squares {
		if containsText(text, target) {
			score += 0.15
		}
	}
	for _, commitment := range mem.Commitments {
		if containsText(text, commitment) {
			score += 0.05
		}
	}
	if candidate.LegalMove.Capture {
		score += 0.05
	}
	if candidate.LegalMove.Check {
		score += 0.08
	}
	return math.Max(-1, math.Min(1, score))
}

func containsText(text, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(needle))
}

func lastMove(history []chesscore.MoveRecord) string {
	if len(history) == 0 {
		return ""
	}
	return history[len(history)-1].UCI
}

func verifierName(v *verifier.Result) string {
	if v == nil {
		return ""
	}
	return v.Name
}

func remainingTimingMS(total int64, parts ...int64) int64 {
	remaining := total
	for _, part := range parts {
		remaining -= part
	}
	if remaining < 0 {
		return 0
	}
	return remaining
}
