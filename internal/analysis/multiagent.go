package analysis

import (
	"fmt"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const MultiAgentReviewSchemaVersion = "multi-agent-review.v1"

type MultiAgentReview struct {
	SchemaVersion string        `json:"schema_version"`
	GameID        string        `json:"game_id,omitempty"`
	Ply           int           `json:"ply,omitempty"`
	SelectedMove  string        `json:"selected_move,omitempty"`
	Arbiter       string        `json:"arbiter"`
	Reviews       []AgentReview `json:"reviews"`
}

type AgentReview struct {
	Role       string   `json:"role"`
	Summary    string   `json:"summary"`
	Move       string   `json:"move,omitempty"`
	Confidence float64  `json:"confidence"`
	Findings   []string `json:"findings,omitempty"`
}

func ReviewDecision(dec *decision.MoveDecision) MultiAgentReview {
	if dec == nil {
		return MultiAgentReview{
			SchemaVersion: MultiAgentReviewSchemaVersion,
			Arbiter:       "No decision is available for multi-agent review.",
			Reviews: []AgentReview{
				{Role: "strategist", Summary: "Waiting for a decision trace.", Confidence: 0.2},
				{Role: "critic", Summary: "No candidates to critique yet.", Confidence: 0.2},
				{Role: "tactician", Summary: "No verifier or board evidence yet.", Confidence: 0.2},
				{Role: "arbiter", Summary: "Run analysis or request an engine move first.", Confidence: 0.2},
			},
		}
	}
	selected := dec.SelectedMove.UCI
	if dec.SelectedMove.SAN != "" {
		selected = dec.SelectedMove.SAN + " (" + dec.SelectedMove.UCI + ")"
	}
	coherence := strategy.EvaluatePlanCoherence(dec.StrategyAfter)
	diversity := strategy.EvaluateCandidateDiversity(dec.CandidateMoves)
	reviews := []AgentReview{
		strategistReview(dec, coherence),
		criticReview(dec, diversity),
		tacticianReview(dec),
	}
	arbiter := arbiterReview(dec, coherence, diversity)
	reviews = append(reviews, arbiter)
	return MultiAgentReview{
		SchemaVersion: MultiAgentReviewSchemaVersion,
		GameID:        dec.GameID,
		Ply:           dec.Ply,
		SelectedMove:  selected,
		Arbiter:       arbiter.Summary,
		Reviews:       reviews,
	}
}

func strategistReview(dec *decision.MoveDecision, coherence strategy.PlanCoherenceReport) AgentReview {
	findings := []string{
		fmt.Sprintf("Plan status is %s with %.2f confidence.", dec.StrategyAfter.Plan.Status, dec.StrategyAfter.Plan.Confidence),
		fmt.Sprintf("Plan coherence is %s at %.2f.", coherence.Status, coherence.Score),
	}
	if dec.StrategyDiff.Reason != "" {
		findings = append(findings, dec.StrategyDiff.Reason)
	}
	return AgentReview{
		Role:       "strategist",
		Summary:    "Checks whether the selected move advances the persistent plan.",
		Move:       dec.SelectedMove.UCI,
		Confidence: coherence.Score,
		Findings:   findings,
	}
}

func criticReview(dec *decision.MoveDecision, diversity strategy.CandidateDiversityReport) AgentReview {
	findings := []string{
		fmt.Sprintf("%d candidate(s) reviewed; diversity is %s at %.2f.", diversity.CandidateCount, diversity.Status, diversity.Score),
	}
	if dec.FallbackUsed {
		findings = append(findings, "Fallback was used; inspect provider reliability and prompt validity.")
	}
	rejected := 0
	warnings := 0
	for _, candidate := range dec.CandidateMoves {
		switch candidate.VerifierScore.Status {
		case "rejected":
			rejected++
		case "warning":
			warnings++
		}
	}
	if rejected > 0 || warnings > 0 {
		findings = append(findings, fmt.Sprintf("Verifier marked %d rejected and %d warning candidate(s).", rejected, warnings))
	}
	return AgentReview{
		Role:       "critic",
		Summary:    "Looks for narrow candidate sets, fallback paths, and rejected alternatives.",
		Move:       dec.SelectedMove.UCI,
		Confidence: 0.5 + diversity.Score/2,
		Findings:   findings,
	}
}

func tacticianReview(dec *decision.MoveDecision) AgentReview {
	findings := []string{}
	if dec.VerifierTrace != nil {
		status := "not used"
		if dec.VerifierTrace.Used {
			status = "used"
		}
		findings = append(findings, fmt.Sprintf("Verifier %s was %s.", dec.VerifierTrace.Name, status))
	}
	if dec.Assistance.SearchUsed {
		findings = append(findings, "Hybrid search signal was included: "+dec.Assistance.SearchName+".")
	}
	if dec.SelectedMove.Capture {
		findings = append(findings, "Selected move is a capture.")
	}
	if dec.SelectedMove.Check {
		findings = append(findings, "Selected move gives check.")
	}
	if len(findings) == 0 {
		findings = append(findings, "No tactical warnings were present in the trace.")
	}
	confidence := 0.55
	if dec.VerifierTrace != nil && dec.VerifierTrace.Used {
		confidence += 0.25
	}
	if dec.Assistance.SearchUsed {
		confidence += 0.15
	}
	if confidence > 1 {
		confidence = 1
	}
	return AgentReview{
		Role:       "tactician",
		Summary:    "Reviews tactical safety and disclosed engine assistance.",
		Move:       dec.SelectedMove.UCI,
		Confidence: confidence,
		Findings:   findings,
	}
}

func arbiterReview(dec *decision.MoveDecision, coherence strategy.PlanCoherenceReport, diversity strategy.CandidateDiversityReport) AgentReview {
	confidence := (coherence.Score + diversity.Score) / 2
	if dec.VerifierTrace != nil && dec.VerifierTrace.Used {
		confidence += 0.10
	}
	if dec.FallbackUsed {
		confidence -= 0.20
	}
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	summary := fmt.Sprintf("Approve %s with %.2f review confidence.", dec.SelectedMove.UCI, confidence)
	if dec.FallbackUsed {
		summary = fmt.Sprintf("Approve fallback %s only as a safe legal continuation; review is required.", dec.SelectedMove.UCI)
	}
	return AgentReview{
		Role:       "arbiter",
		Summary:    summary,
		Move:       dec.SelectedMove.UCI,
		Confidence: confidence,
		Findings: []string{
			"Combines strategist, critic, and tactician signals into one review.",
		},
	}
}
