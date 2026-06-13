package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	CompressedMemorySchemaVersion      = "strategy-memory-compressed.v1"
	PlanCoherenceSchemaVersion         = "plan-coherence.v1"
	CandidateDiversitySchemaVersion    = "candidate-diversity.v1"
	defaultCompressedMemoryRetainItems = 4
)

type CompressedMemory struct {
	SchemaVersion   string   `json:"schema_version"`
	MemoryID        string   `json:"memory_id"`
	GameID          string   `json:"game_id"`
	Side            string   `json:"side"`
	SourceHash      string   `json:"source_hash"`
	Ply             int      `json:"ply"`
	Phase           string   `json:"phase"`
	PlanSummary     string   `json:"plan_summary"`
	PlanStatus      string   `json:"plan_status"`
	PlanConfidence  float64  `json:"plan_confidence"`
	CriticalTargets []string `json:"critical_targets,omitempty"`
	Commitments     []string `json:"commitments,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Triggers        []string `json:"triggers,omitempty"`
	StyleNotes      []string `json:"style_notes,omitempty"`
	RetainedItems   int      `json:"retained_items"`
	DroppedItems    int      `json:"dropped_items"`
}

type PlanCoherenceReport struct {
	SchemaVersion string          `json:"schema_version"`
	Score         float64         `json:"score"`
	Status        string          `json:"status"`
	Findings      []StrategyAlert `json:"findings,omitempty"`
}

type CandidateDiversityReport struct {
	SchemaVersion  string       `json:"schema_version"`
	CandidateCount int          `json:"candidate_count"`
	Score          float64      `json:"score"`
	Status         string       `json:"status"`
	Families       []MoveFamily `json:"families,omitempty"`
	Warnings       []string     `json:"warnings,omitempty"`
}

type MoveFamily struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func CompressMemory(mem StrategyMemory, maxItems int) CompressedMemory {
	if maxItems <= 0 {
		maxItems = defaultCompressedMemoryRetainItems
	}
	targets := append(append([]string{}, mem.Targets.Squares...), mem.Targets.Pieces...)
	targets = append(targets, mem.Targets.Pawns...)
	triggers := make([]string, 0, len(mem.RefutationTriggers))
	for _, trigger := range mem.RefutationTriggers {
		text := strings.TrimSpace(trigger.Condition)
		if trigger.Response != "" {
			text += " -> " + strings.TrimSpace(trigger.Response)
		}
		triggers = append(triggers, text)
	}
	rawItemCount := len(targets) + len(mem.Commitments) + len(mem.TacticalWarnings) + len(triggers) + len(mem.StyleNotes)
	out := CompressedMemory{
		SchemaVersion:  CompressedMemorySchemaVersion,
		MemoryID:       mem.MemoryID,
		GameID:         mem.GameID,
		Side:           mem.Side,
		SourceHash:     HashMemory(mem),
		Ply:            mem.Ply,
		Phase:          mem.Phase,
		PlanSummary:    bounded(mem.Plan.Summary, 280),
		PlanStatus:     mem.Plan.Status,
		PlanConfidence: round2(clamp01(mem.Plan.Confidence)),
	}
	out.CriticalTargets = limitStrings(targets, maxItems, 80)
	out.Commitments = limitStrings(mem.Commitments, maxItems, 120)
	out.Warnings = limitStrings(mem.TacticalWarnings, maxItems, 140)
	out.Triggers = limitStrings(triggers, maxItems, 160)
	out.StyleNotes = limitStrings(mem.StyleNotes, maxItems, 120)
	out.RetainedItems = len(out.CriticalTargets) + len(out.Commitments) + len(out.Warnings) + len(out.Triggers) + len(out.StyleNotes)
	if rawItemCount > out.RetainedItems {
		out.DroppedItems = rawItemCount - out.RetainedItems
	}
	return out
}

func EvaluatePlanCoherence(mem StrategyMemory) PlanCoherenceReport {
	score := 1.0
	findings := []StrategyAlert{}
	penalize := func(points float64, code, severity, message string) {
		score -= points
		findings = append(findings, StrategyAlert{Code: code, Severity: severity, Message: message})
	}
	if strings.TrimSpace(mem.Plan.Summary) == "" {
		penalize(0.28, "coherence_plan_missing", "high", "The strategy has no plan summary.")
	}
	if strings.TrimSpace(mem.Phase) == "" || mem.Phase == "unknown" {
		penalize(0.12, "coherence_phase_unknown", "medium", "The strategy phase is unknown.")
	}
	if len(mem.Targets.Squares)+len(mem.Targets.Pieces)+len(mem.Targets.Pawns) == 0 && mem.Ply >= 4 {
		penalize(0.14, "coherence_targets_missing", "medium", "The plan has no concrete targets after the opening moves.")
	}
	if len(mem.Commitments) == 0 && mem.Ply >= 6 {
		penalize(0.10, "coherence_commitments_missing", "medium", "No plan commitments are recorded for the current line.")
	}
	if len(mem.RefutationTriggers) == 0 && mem.Ply >= 6 {
		penalize(0.12, "coherence_triggers_missing", "medium", "No refutation triggers are recorded.")
	}
	if mem.Plan.Confidence > 0 && mem.OpponentModel.Confidence > 0 && math.Abs(mem.Plan.Confidence-mem.OpponentModel.Confidence) > 0.55 {
		penalize(0.08, "coherence_confidence_gap", "low", "Plan and opponent-model confidence diverge sharply.")
	}
	if strings.EqualFold(mem.Plan.Status, "continue") && len(mem.TacticalWarnings) >= 4 {
		penalize(0.10, "coherence_continue_despite_warnings", "medium", "The plan is marked continue despite several tactical warnings.")
	}
	score = round2(clamp01(score))
	return PlanCoherenceReport{
		SchemaVersion: PlanCoherenceSchemaVersion,
		Score:         score,
		Status:        coherenceStatus(score),
		Findings:      findings,
	}
}

func EvaluateCandidateDiversity(candidates []CandidateMove) CandidateDiversityReport {
	report := CandidateDiversityReport{
		SchemaVersion:  CandidateDiversitySchemaVersion,
		CandidateCount: len(candidates),
	}
	if len(candidates) == 0 {
		report.Status = "empty"
		report.Warnings = []string{"No candidate moves were available for diversity scoring."}
		return report
	}
	families := map[string]int{}
	destinations := map[string]struct{}{}
	purposes := map[string]struct{}{}
	for _, candidate := range candidates {
		family := candidateFamily(candidate)
		families[family]++
		if len(candidate.UCI) >= 4 {
			destinations[candidate.UCI[2:4]] = struct{}{}
		}
		for _, token := range strings.Fields(strings.ToLower(candidate.Purpose)) {
			if len(token) > 4 {
				purposes[token] = struct{}{}
			}
		}
	}
	for name, count := range families {
		report.Families = append(report.Families, MoveFamily{Name: name, Count: count})
	}
	sort.Slice(report.Families, func(i, j int) bool {
		return report.Families[i].Name < report.Families[j].Name
	})
	familyScore := float64(len(families)) / math.Min(4, float64(len(candidates)))
	destinationScore := float64(len(destinations)) / float64(len(candidates))
	purposeScore := math.Min(1, float64(len(purposes))/float64(len(candidates)+1))
	report.Score = round2(clamp01(0.45*familyScore + 0.35*destinationScore + 0.20*purposeScore))
	report.Status = diversityStatus(report.Score)
	if report.Score < 0.45 {
		report.Warnings = append(report.Warnings, "Candidate set is narrow; prompt or provider diversity should be reviewed.")
	}
	if len(candidates) < 3 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Only %d candidate move(s) were retained.", len(candidates)))
	}
	return report
}

func candidateFamily(candidate CandidateMove) string {
	switch {
	case candidate.LegalMove.Check:
		return "check"
	case candidate.LegalMove.Capture:
		return "capture"
	case candidate.LegalMove.Promotion != "":
		return "promotion"
	case strings.Contains(strings.ToLower(candidate.Purpose), "castle"):
		return "king_safety"
	default:
		return "quiet"
	}
}

func coherenceStatus(score float64) string {
	switch {
	case score >= 0.82:
		return "coherent"
	case score >= 0.62:
		return "needs_review"
	default:
		return "weak"
	}
}

func diversityStatus(score float64) string {
	switch {
	case score >= 0.70:
		return "broad"
	case score >= 0.45:
		return "mixed"
	default:
		return "narrow"
	}
}
