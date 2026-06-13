package analysis

import (
	"testing"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

func TestReviewDecisionBuildsCoreRoles(t *testing.T) {
	mem := strategy.NewMemory("game", "white")
	mem.Ply = 8
	mem.Phase = "middlegame"
	mem.Targets.Squares = []string{"e5"}
	mem.Commitments = []string{"Keep central pressure."}
	mem.RefutationTriggers = []strategy.RefutationTrigger{{Condition: "queen trade", Response: "reassess"}}
	dec := &decision.MoveDecision{
		GameID:        "game",
		Ply:           9,
		SelectedMove:  chesscore.LegalMove{UCI: "g1f3", SAN: "Nf3"},
		StrategyAfter: mem,
		CandidateMoves: []strategy.CandidateMove{
			{UCI: "g1f3", Purpose: "develop", VerifierScore: strategy.VerifierScore{Status: "accepted"}},
			{UCI: "e4d5", Purpose: "capture", LegalMove: chesscore.LegalMove{Capture: true}, VerifierScore: strategy.VerifierScore{Status: "warning"}},
		},
		VerifierTrace: &verifier.Result{Name: "static_safety", Used: true},
	}

	review := ReviewDecision(dec)
	if review.SchemaVersion != MultiAgentReviewSchemaVersion || review.GameID != "game" || review.SelectedMove == "" {
		t.Fatalf("review missing boundary fields: %+v", review)
	}
	for _, role := range []string{"strategist", "critic", "tactician", "arbiter"} {
		if !hasRole(review.Reviews, role) {
			t.Fatalf("missing role %s in %+v", role, review.Reviews)
		}
	}
	if review.Arbiter == "" {
		t.Fatalf("missing arbiter summary: %+v", review)
	}
}

func TestReviewDecisionHandlesMissingDecision(t *testing.T) {
	review := ReviewDecision(nil)
	if review.SchemaVersion != MultiAgentReviewSchemaVersion || review.Arbiter == "" || len(review.Reviews) != 4 {
		t.Fatalf("empty review not populated: %+v", review)
	}
}

func hasRole(reviews []AgentReview, role string) bool {
	for _, review := range reviews {
		if review.Role == role {
			return true
		}
	}
	return false
}
