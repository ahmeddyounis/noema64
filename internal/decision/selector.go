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
	start := time.Now()
	decisionID := "dec_" + uuid.NewString()
	stages := newStageRecorder(start, decisionID, req.Progress)
	if req.Game == nil {
		return nil, fmt.Errorf("nil game")
	}
	readStage := stages.begin("reading_position", "Snapshot position, legal moves, and deterministic features.")
	legal := req.Game.LegalMoves()
	if len(legal) == 0 {
		readStage.finish("failed", "No legal moves are available.")
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

	snapshot := req.Game.Snapshot()
	stages.setGameID(snapshot.GameID)
	readStage.finish("completed", fmt.Sprintf("%d legal moves found.", len(legal)))
	memBefore := req.Memory
	memoryStage := stages.begin("updating_strategy_memory", "Prepare current strategy memory and prompt context.")
	stratReq := strategy.StrategyRequest{
		GameID:             snapshot.GameID,
		FEN:                snapshot.FEN,
		PGN:                snapshot.PGN,
		SideToMove:         snapshot.SideToMove,
		MoveNumber:         snapshot.Ply/2 + 1,
		LastOpponentMove:   lastMove(snapshot.MoveHistory),
		LegalMoves:         legal,
		Features:           req.Game.Features(),
		PreviousMemory:     req.Memory,
		Mode:               req.Mode,
		Personality:        req.Personality,
		PersonalityProfile: req.PersonalityProfile,
	}
	system, user, err := strategy.BuildPrompt(stratReq)
	if err != nil {
		memoryStage.finish("failed", err.Error())
		return fallbackDecision(req, decisionID, memBefore, "prompt_build_error", start, nil, stages.snapshot()), nil
	}
	memoryStage.finish("completed", "Prompt context prepared.")

	providerCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	providerStage := stages.begin("asking_provider", "Request structured candidate moves from provider.")
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
			"fen":            snapshot.FEN,
		},
	})
	providerMS := time.Since(providerStart).Milliseconds()
	providerTrace := ProviderTrace{
		Name:                  req.Provider.Name(),
		Model:                 req.Model,
		PromptID:              strategy.PromptID,
		PromptVersion:         strategy.PromptVersion,
		PromptSchemaVersion:   strategy.PromptTemplateSchemaVersion,
		DecisionSchemaVersion: strategy.DecisionSchemaVersion,
		Temperature:           req.Temperature,
		MaxTokens:             req.MaxTokens,
		RetryCount:            req.ProviderRetries,
	}
	if req.LogRawPrompts {
		providerTrace.RawPrompt = &PromptTrace{System: system, User: user}
	}
	if err != nil {
		providerStage.finish("failed", err.Error())
		providerTrace.Error = err.Error()
		return fallbackDecision(req, decisionID, memBefore, "provider_error", start, &providerTrace, stages.snapshot()), nil
	}
	providerStage.finish("completed", fmt.Sprintf("Provider returned in %d ms.", providerMS))
	providerTrace.RawAvailable = resp.RawAvailable
	providerTrace.Name = resp.Provider
	providerTrace.Model = resp.Model
	if req.LogRawResponse {
		providerTrace.RawResponse = resp.Text
	}

	repairStage := stages.begin("repairing_candidate_moves", "Parse provider JSON and normalize candidate moves.")
	parse := strategy.ParseDecision(resp.Text)
	providerTrace.ParseStatus = parse.Status
	if parse.Status != "ok" && parse.Status != "extracted_json" {
		repairStage.finish("failed", parse.Error)
		providerTrace.Error = parse.Error
		return fallbackDecision(req, decisionID, memBefore, "provider_schema_invalid", start, &providerTrace, stages.snapshot()), nil
	}
	providerTrace.ParsedDecision = &parse.Decision

	candidates, repairs := strategy.NormalizeCandidates(req.Game, parse.Decision.CandidateMoves)
	if len(candidates) == 0 {
		repairStage.finish("failed", "No legal provider candidates remained after normalization.")
		return fallbackDecision(req, decisionID, memBefore, "no_legal_llm_candidates", start, &providerTrace, stages.snapshot()), nil
	}
	if len(candidates) > req.MaxCandidates {
		candidates = candidates[:req.MaxCandidates]
	}
	repairStage.finish("completed", fmt.Sprintf("%d legal candidates retained.", len(candidates)))

	verifyStage := stages.begin("verifying_tactics", "Run verifier or legal-only safety checks.")
	verifierStart := time.Now()
	activeVerifier := req.Verifier
	if req.Mode == strategy.ModePure {
		activeVerifier = verifier.LegalOnlyVerifier{}
	}
	verifyResult, err := activeVerifier.VerifyCandidates(ctx, verifier.Request{
		Game:       req.Game.Clone(),
		FEN:        snapshot.FEN,
		Candidates: candidates,
		Mode:       req.Mode,
	})
	verifierMS := time.Since(verifierStart).Milliseconds()
	if err != nil {
		verifyResult = &verifier.Result{Enabled: req.Mode != strategy.ModePure, Used: false, Name: activeVerifier.Name(), Error: err.Error()}
		verifyStage.finish("failed", err.Error())
	} else {
		verifyStage.finish("completed", fmt.Sprintf("Verifier completed in %d ms.", verifierMS))
	}
	applyVerifierScores(candidates, verifyResult)
	scoreStage := stages.begin("scoring_candidates", "Combine LLM, plan, tactical, and search signals.")
	searchStart := time.Now()
	searchUsed, searchName := applySearchScores(ctx, req.Game, candidates, req.Mode)
	searchMS := time.Since(searchStart).Milliseconds()
	scoreCandidates(candidates, req.Memory, req.Mode, strategy.ResolvePersonalityProfile(req.Personality, req.PersonalityProfile))
	chosen, ok := selectCandidate(candidates, req.Mode)
	if !ok {
		scoreStage.finish("failed", "No acceptable candidate after scoring.")
		return fallbackDecision(req, decisionID, memBefore, "arbiter_no_move", start, &providerTrace, stages.snapshot()), nil
	}
	scoreStage.finish("completed", fmt.Sprintf("Selected %s.", chosen.UCI))

	updateStage := stages.begin("updating_strategy_memory_after_move", "Merge provider strategy update into persistent memory.")
	memAfter := strategy.MergeMemory(memBefore, parse.Decision.StrategyUpdate, parse.Decision.PreviousPlanStatus, snapshot.GameID, snapshot.SideToMove, snapshot.Ply+1, decisionID, chosen.UCI)
	diff := strategy.DiffMemory(memBefore, memAfter)
	updateStage.finish("completed", "Strategy memory updated.")
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
		Stages: stages.snapshot(),
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

func fallbackDecision(req Request, decisionID string, mem strategy.StrategyMemory, reason string, start time.Time, providerTrace *ProviderTrace, stages []StageTrace) *MoveDecision {
	if providerTrace == nil {
		providerTrace = &ProviderTrace{
			Name:                  req.Provider.Name(),
			Model:                 req.Model,
			PromptID:              strategy.PromptID,
			PromptVersion:         strategy.PromptVersion,
			PromptSchemaVersion:   strategy.PromptTemplateSchemaVersion,
			DecisionSchemaVersion: strategy.DecisionSchemaVersion,
			Temperature:           req.Temperature,
			MaxTokens:             req.MaxTokens,
			RetryCount:            req.ProviderRetries,
		}
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
	memAfter := strategy.MergeMemory(mem, update, "modify", snapshot.GameID, snapshot.SideToMove, snapshot.Ply+1, decisionID, mv.UCI)
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
	fallbackStarted := time.Now()
	fallbackFinished := fallbackStarted
	stages = append(append([]StageTrace(nil), stages...), CompletedStage("fallback", "completed", "Fallback selected a legal move after "+reason+".", fallbackStarted, fallbackFinished))
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
		Stages:             stages,
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

func scoreCandidates(candidates []strategy.CandidateMove, mem strategy.StrategyMemory, mode strategy.EngineMode, profile strategy.PersonalityProfile) {
	for i := range candidates {
		plan := planAlignment(candidates[i], mem)
		candidates[i].PlanAlignmentScore = plan
		personalityFit := personalityAlignment(candidates[i], profile)
		candidates[i].PersonalityScore = personalityFit
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
			candidates[i].FinalScore = 0.20*llm + 0.20*plan + 0.20*tactical + 0.35*candidates[i].SearchScore + 0.03*personalityFit
		case strategy.ModeBlunderguard:
			candidates[i].FinalScore = 0.35*llm + 0.25*plan + 0.30*tactical + 0.05*personalityFit
		default:
			candidates[i].FinalScore = 0.55*llm + 0.35*plan + 0.05*personalityFit
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

func personalityAlignment(candidate strategy.CandidateMove, profile strategy.PersonalityProfile) float64 {
	riskDelta := profile.RiskTolerance - 0.45
	score := 0.0
	forcing := candidate.LegalMove.Capture || candidate.LegalMove.Check
	if candidate.LegalMove.Capture {
		score += riskDelta * 0.9
	}
	if candidate.LegalMove.Check {
		score += riskDelta * 0.7
	}
	if !forcing {
		score += -riskDelta * 0.4
	}
	switch candidate.VerifierScore.Status {
	case "warning":
		score -= math.Max(0, 0.60-profile.RiskTolerance) * 0.5
	case "rejected":
		score -= math.Max(0.20, 0.80-profile.RiskTolerance)
	}
	switch profile.ID {
	case strategy.PersonalityAggressive:
		if forcing {
			score += 0.25
		} else {
			score -= 0.05
		}
	case strategy.PersonalityPositional:
		if candidate.PlanAlignmentScore >= 0.30 {
			score += 0.20
		}
		if forcing {
			score -= 0.10
		}
	case strategy.PersonalityBeginnerCoach:
		if !forcing {
			score += 0.15
		}
		riskText := strings.ToLower(candidate.Risk + " " + candidate.Purpose)
		for _, word := range []string{"sacrifice", "speculative", "unclear", "complex"} {
			if strings.Contains(riskText, word) {
				score -= 0.15
				break
			}
		}
	}
	return math.Max(-1, math.Min(1, score))
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
