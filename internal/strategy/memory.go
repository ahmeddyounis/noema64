package strategy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

func NewMemory(gameID, side string) StrategyMemory {
	return StrategyMemory{
		SchemaVersion: MemorySchemaVersion,
		MemoryID:      "mem_" + uuid.NewString(),
		GameID:        gameID,
		Side:          side,
		Phase:         "unknown",
		Plan: Plan{
			Summary:      "No strategy yet. Noema64 will create a plan after its first decision.",
			Status:       "new",
			Confidence:   0.5,
			HorizonMoves: 4,
		},
		Targets: Targets{},
		OpponentModel: OpponentModel{
			LikelyPlan: "Unknown until more moves are played.",
			Confidence: 0.2,
		},
		StyleNotes: []string{"Prefer clear development, king safety, and legal candidate moves."},
	}
}

func MergeMemory(prev StrategyMemory, update StrategyUpdate, status string, gameID, side string, ply int, decisionID, moveUCI string) StrategyMemory {
	next := prev
	next.SchemaVersion = MemorySchemaVersion
	next.GameID = gameID
	next.Side = side
	next.Ply = ply
	if next.MemoryID == "" {
		next.MemoryID = "mem_" + uuid.NewString()
	}
	if update.Phase != "" {
		next.Phase = bounded(update.Phase, 32)
	}
	if update.PlanSummary != "" {
		next.Plan.Summary = bounded(update.PlanSummary, 600)
	}
	if update.Confidence >= 0 {
		next.Plan.Confidence = clamp(update.Confidence, 0, 1)
	}
	if next.Plan.HorizonMoves == 0 {
		next.Plan.HorizonMoves = 4
	}
	if update.LastUpdateSummary != "" {
		next.LastUpdate.Summary = bounded(update.LastUpdateSummary, 500)
	}
	next.Plan.Status = planStatus(status, update)
	next.Targets.Squares = limitStrings(update.MainTargets, 12, 64)
	next.PieceImprovement = stringsToPieceImprovements(update.PieceImprovement)
	next.PawnBreaks = stringsToPawnBreaks(update.PawnBreaks)
	next.OpponentModel.LikelyPlan = bounded(update.OpponentPlanGuess, 500)
	next.Commitments = limitStrings(update.Commitments, 8, 140)
	next.RefutationTriggers = stringsToTriggers(update.RefutationTriggers)
	next.TacticalWarnings = limitStrings(update.TacticalWarnings, 8, 180)
	next.LastUpdate.DecisionID = decisionID
	next.LastUpdate.MovePlayed = moveUCI
	return next
}

func DiffMemory(before, after StrategyMemory) MemoryDiff {
	diff := MemoryDiff{
		PlanBefore:      before.Plan.Summary,
		PlanAfter:       after.Plan.Summary,
		ConfidenceDelta: after.Plan.Confidence - before.Plan.Confidence,
		Reason:          after.LastUpdate.Summary,
	}
	if before.Plan.Summary != after.Plan.Summary {
		diff.ChangedFields = append(diff.ChangedFields, "plan.summary")
	}
	if before.Plan.Status != after.Plan.Status {
		diff.ChangedFields = append(diff.ChangedFields, "plan.status")
	}
	if before.Plan.Confidence != after.Plan.Confidence {
		diff.ChangedFields = append(diff.ChangedFields, "plan.confidence")
	}
	if strings.Join(before.TacticalWarnings, "\x00") != strings.Join(after.TacticalWarnings, "\x00") {
		diff.ChangedFields = append(diff.ChangedFields, "tactical_warnings")
	}
	if strings.Join(before.Commitments, "\x00") != strings.Join(after.Commitments, "\x00") {
		diff.ChangedFields = append(diff.ChangedFields, "commitments")
	}
	return diff
}

func HashMemory(mem StrategyMemory) string {
	b, _ := json.Marshal(mem)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

func planStatus(status string, update StrategyUpdate) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "continue", "modify", "abandon", "new":
		return strings.ToLower(strings.TrimSpace(status))
	}
	switch strings.TrimSpace(update.PlanSummary) {
	case "":
		return "continue"
	default:
		return "modify"
	}
}

func stringsToPieceImprovements(items []string) []PieceImprovement {
	items = limitStrings(items, 8, 160)
	out := make([]PieceImprovement, 0, len(items))
	for _, item := range items {
		out = append(out, PieceImprovement{Piece: item, Problem: "improve activity", Priority: 0.5, Reason: item})
	}
	return out
}

func stringsToPawnBreaks(items []string) []PawnBreak {
	items = limitStrings(items, 6, 120)
	out := make([]PawnBreak, 0, len(items))
	for _, item := range items {
		out = append(out, PawnBreak{MovePattern: item, Purpose: "advance the current plan", Risk: "verify tactically before committing"})
	}
	return out
}

func stringsToTriggers(items []string) []RefutationTrigger {
	items = limitStrings(items, 6, 180)
	out := make([]RefutationTrigger, 0, len(items))
	for _, item := range items {
		out = append(out, RefutationTrigger{Condition: item, Response: "Reassess the plan and legal candidate moves."})
	}
	return out
}

func limitStrings(items []string, maxItems int, maxLen int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = bounded(item, maxLen)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
		if len(out) == maxItems {
			break
		}
	}
	return out
}

func bounded(s string, maxLen int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\x00", ""))
	if len(s) > maxLen {
		return strings.TrimSpace(s[:maxLen])
	}
	return s
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
