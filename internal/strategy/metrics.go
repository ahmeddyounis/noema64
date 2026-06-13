package strategy

import (
	"math"
	"strings"
)

const MemoryMetricsSchemaVersion = "strategy-memory-metrics.v1"

type MemoryMetrics struct {
	SchemaVersion string          `json:"schema_version"`
	Quality       float64         `json:"quality"`
	Completeness  float64         `json:"completeness"`
	Consistency   float64         `json:"consistency"`
	Drift         float64         `json:"drift"`
	AlertLevel    string          `json:"alert_level"`
	Alerts        []StrategyAlert `json:"alerts,omitempty"`
}

type StrategyAlert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func EvaluateMemory(current StrategyMemory, previous *StrategyMemory) MemoryMetrics {
	completeness := memoryCompleteness(current)
	consistency, consistencyAlerts := memoryConsistency(current)
	drift := memoryDrift(current, previous)
	alerts := append([]StrategyAlert{}, consistencyAlerts...)
	if completeness < 0.65 {
		alerts = append(alerts, StrategyAlert{
			Code:     "strategy_memory_incomplete",
			Severity: "medium",
			Message:  "Strategy memory is missing enough structured fields to make the plan easy to audit.",
		})
	}
	if drift >= 0.65 {
		alerts = append(alerts, StrategyAlert{
			Code:     "strategy_drift_high",
			Severity: "high",
			Message:  "The current plan diverged sharply from the previous strategy state.",
		})
	} else if drift >= 0.35 {
		alerts = append(alerts, StrategyAlert{
			Code:     "strategy_drift_medium",
			Severity: "medium",
			Message:  "The current plan changed materially from the previous strategy state.",
		})
	}
	if strings.EqualFold(strings.TrimSpace(current.Plan.Status), "abandon") {
		alerts = append(alerts, StrategyAlert{
			Code:     "strategy_plan_abandoned",
			Severity: "high",
			Message:  "The plan was abandoned and should be reviewed against the last tactical trigger.",
		})
	}
	if current.Ply >= 6 && len(current.RefutationTriggers) == 0 {
		alerts = append(alerts, StrategyAlert{
			Code:     "missing_refutation_triggers",
			Severity: "medium",
			Message:  "No refutation triggers are recorded for a developed position.",
		})
	}
	if current.Plan.Confidence > 0 && current.Plan.Confidence < 0.35 {
		alerts = append(alerts, StrategyAlert{
			Code:     "low_strategy_confidence",
			Severity: "medium",
			Message:  "Strategy confidence is low; analysis or replanning is recommended.",
		})
	}
	quality := clamp01((completeness*0.45 + consistency*0.45 + (1-drift)*0.10))
	return MemoryMetrics{
		SchemaVersion: MemoryMetricsSchemaVersion,
		Quality:       round2(quality),
		Completeness:  round2(completeness),
		Consistency:   round2(consistency),
		Drift:         round2(drift),
		AlertLevel:    alertLevel(alerts),
		Alerts:        alerts,
	}
}

func memoryCompleteness(mem StrategyMemory) float64 {
	score := 0.0
	total := 11.0
	if strings.TrimSpace(mem.SchemaVersion) != "" {
		score++
	}
	if strings.TrimSpace(mem.GameID) != "" {
		score++
	}
	if strings.TrimSpace(mem.Side) != "" {
		score++
	}
	if strings.TrimSpace(mem.Phase) != "" && mem.Phase != "unknown" {
		score++
	}
	if strings.TrimSpace(mem.Plan.Summary) != "" {
		score++
	}
	if strings.TrimSpace(mem.Plan.Status) != "" {
		score++
	}
	if mem.Plan.HorizonMoves > 0 {
		score++
	}
	if len(mem.Targets.Squares)+len(mem.Targets.Pieces)+len(mem.Targets.Pawns) > 0 {
		score++
	}
	if strings.TrimSpace(mem.OpponentModel.LikelyPlan) != "" {
		score++
	}
	if len(mem.Commitments)+len(mem.RefutationTriggers)+len(mem.TacticalWarnings) > 0 {
		score++
	}
	if mem.Ply == 0 || strings.TrimSpace(mem.LastUpdate.DecisionID) != "" || strings.TrimSpace(mem.LastUpdate.Summary) != "" {
		score++
	}
	return clamp01(score / total)
}

func memoryConsistency(mem StrategyMemory) (float64, []StrategyAlert) {
	score := 1.0
	alerts := []StrategyAlert{}
	penalize := func(points float64, code, severity, message string) {
		score -= points
		alerts = append(alerts, StrategyAlert{Code: code, Severity: severity, Message: message})
	}
	if mem.SchemaVersion != MemorySchemaVersion {
		penalize(0.20, "strategy_schema_mismatch", "high", "Strategy memory schema version does not match this release.")
	}
	switch strings.ToLower(strings.TrimSpace(mem.Side)) {
	case "white", "black":
	default:
		penalize(0.12, "strategy_side_invalid", "medium", "Strategy memory side is missing or invalid.")
	}
	switch strings.ToLower(strings.TrimSpace(mem.Plan.Status)) {
	case "new", "continue", "modify", "abandon":
	default:
		penalize(0.12, "strategy_status_invalid", "medium", "Strategy plan status is missing or invalid.")
	}
	if mem.Plan.Confidence < 0 || mem.Plan.Confidence > 1 {
		penalize(0.15, "strategy_confidence_invalid", "medium", "Strategy confidence is outside the 0..1 range.")
	}
	if mem.Plan.HorizonMoves < 0 {
		penalize(0.10, "strategy_horizon_invalid", "medium", "Strategy horizon cannot be negative.")
	}
	if mem.Ply > 0 && strings.TrimSpace(mem.LastUpdate.MovePlayed) == "" {
		penalize(0.08, "strategy_last_move_missing", "low", "Strategy memory has played plies but no last move reference.")
	}
	if duplicateStrings(mem.Commitments) {
		penalize(0.05, "strategy_duplicate_commitments", "low", "Strategy memory contains duplicate commitments.")
	}
	if duplicateTriggers(mem.RefutationTriggers) {
		penalize(0.05, "strategy_duplicate_triggers", "low", "Strategy memory contains duplicate refutation triggers.")
	}
	return clamp01(score), alerts
}

func memoryDrift(current StrategyMemory, previous *StrategyMemory) float64 {
	if previous == nil || strings.TrimSpace(previous.MemoryID) == "" {
		return 0
	}
	diff := DiffMemory(*previous, current)
	score := float64(len(diff.ChangedFields)) * 0.11
	score += math.Abs(diff.ConfidenceDelta) * 0.35
	if previous.Plan.Summary != "" && current.Plan.Summary != previous.Plan.Summary {
		score += 0.15
	}
	switch strings.ToLower(strings.TrimSpace(current.Plan.Status)) {
	case "modify":
		score += 0.18
	case "new":
		if previous.Ply > 0 {
			score += 0.22
		}
	case "abandon":
		score += 0.45
	}
	if strings.Join(triggerConditions(current.RefutationTriggers), "\x00") != strings.Join(triggerConditions(previous.RefutationTriggers), "\x00") {
		score += 0.10
	}
	return clamp01(score)
}

func duplicateStrings(items []string) bool {
	seen := map[string]struct{}{}
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

func duplicateTriggers(items []RefutationTrigger) bool {
	return duplicateStrings(triggerConditions(items))
}

func triggerConditions(items []RefutationTrigger) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Condition)
	}
	return out
}

func alertLevel(alerts []StrategyAlert) string {
	level := "none"
	for _, alert := range alerts {
		switch strings.ToLower(alert.Severity) {
		case "high":
			return "high"
		case "medium":
			level = "medium"
		case "low":
			if level == "none" {
				level = "low"
			}
		}
	}
	return level
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
