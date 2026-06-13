package strategy

import "testing"

func TestMergeMemoryHonorsExplicitPlanStatus(t *testing.T) {
	prev := NewMemory("game", "white")
	update := StrategyUpdate{
		PlanSummary:       "Switch to a defensive setup.",
		Phase:             "middlegame",
		Confidence:        0.42,
		LastUpdateSummary: "Opponent refuted the previous plan.",
	}
	for _, status := range []string{"continue", "modify", "abandon", "new"} {
		t.Run(status, func(t *testing.T) {
			merged := MergeMemory(prev, update, status, "game", "white", 12, "dec", "g1f3")
			if merged.Plan.Status != status {
				t.Fatalf("plan status = %q, want %q", merged.Plan.Status, status)
			}
		})
	}
}

func TestMergeMemoryDefaultsPlanStatusForLegacyUpdates(t *testing.T) {
	prev := NewMemory("game", "white")
	merged := MergeMemory(prev, StrategyUpdate{PlanSummary: "Improve pieces."}, "", "game", "white", 1, "dec", "g1f3")
	if merged.Plan.Status != "modify" {
		t.Fatalf("plan status = %q, want modify", merged.Plan.Status)
	}
	continued := MergeMemory(prev, StrategyUpdate{}, "", "game", "white", 1, "dec", "g1f3")
	if continued.Plan.Status != "continue" {
		t.Fatalf("empty update status = %q, want continue", continued.Plan.Status)
	}
}
