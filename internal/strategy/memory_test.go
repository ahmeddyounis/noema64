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

func TestEvaluateMemoryReportsQualityAndDrift(t *testing.T) {
	prev := NewMemory("game", "white")
	prev.Phase = "opening"
	prev.Targets.Squares = []string{"e4"}
	prev.Commitments = []string{"Castle before opening the center."}
	prev.RefutationTriggers = []RefutationTrigger{{Condition: "king safety is compromised", Response: "reassess"}}

	next := prev
	next.Ply = 10
	next.Plan.Summary = "Abandon the kingside plan and defend dark squares."
	next.Plan.Status = "abandon"
	next.Plan.Confidence = 0.25
	next.LastUpdate.MovePlayed = "g1f3"
	next.LastUpdate.Summary = "Opponent refuted the previous plan."

	metrics := EvaluateMemory(next, &prev)
	if metrics.SchemaVersion != MemoryMetricsSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", metrics.SchemaVersion, MemoryMetricsSchemaVersion)
	}
	if metrics.Quality <= 0 || metrics.Completeness <= 0 || metrics.Consistency <= 0 {
		t.Fatalf("metrics should be populated: %+v", metrics)
	}
	if metrics.Drift < 0.65 {
		t.Fatalf("drift = %.2f, want high drift", metrics.Drift)
	}
	if metrics.AlertLevel != "high" || !hasStrategyAlert(metrics.Alerts, "strategy_plan_abandoned") {
		t.Fatalf("missing high abandon alert: %+v", metrics)
	}
}

func TestEvaluateMemoryFlagsInvalidMemory(t *testing.T) {
	mem := StrategyMemory{
		SchemaVersion: "legacy",
		Side:          "north",
		Plan:          Plan{Status: "stale", Confidence: 2, HorizonMoves: -1},
		Commitments:   []string{"same", "same"},
	}
	metrics := EvaluateMemory(mem, nil)
	if metrics.Consistency >= 0.8 {
		t.Fatalf("consistency = %.2f, want degraded score", metrics.Consistency)
	}
	for _, want := range []string{"strategy_schema_mismatch", "strategy_side_invalid", "strategy_duplicate_commitments"} {
		if !hasStrategyAlert(metrics.Alerts, want) {
			t.Fatalf("missing alert %s in %+v", want, metrics.Alerts)
		}
	}
}

func hasStrategyAlert(alerts []StrategyAlert, code string) bool {
	for _, alert := range alerts {
		if alert.Code == code {
			return true
		}
	}
	return false
}
