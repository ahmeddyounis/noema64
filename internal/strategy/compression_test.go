package strategy

import (
	"testing"

	"github.com/ahmedyounis/noema64/internal/chesscore"
)

func TestCompressMemoryRetainsCriticalFields(t *testing.T) {
	mem := NewMemory("game", "white")
	mem.Ply = 12
	mem.Phase = "middlegame"
	mem.Plan.Summary = "Attack the dark squares while keeping the king safe."
	mem.Plan.Status = "continue"
	mem.Targets.Squares = []string{"e5", "f6", "h7"}
	mem.Commitments = []string{"Do not trade the attacking bishop.", "Open the h-file first.", "Avoid queen trades."}
	mem.TacticalWarnings = []string{"Back rank needs luft.", "Knight fork on e2."}
	mem.RefutationTriggers = []RefutationTrigger{{Condition: "queen trade offered", Response: "decline if attack continues"}}

	compressed := CompressMemory(mem, 2)
	if compressed.SchemaVersion != CompressedMemorySchemaVersion || compressed.SourceHash == "" {
		t.Fatalf("compressed memory missing identity fields: %+v", compressed)
	}
	if len(compressed.CriticalTargets) != 2 || len(compressed.Commitments) != 2 || compressed.DroppedItems == 0 {
		t.Fatalf("compression did not limit retained lists: %+v", compressed)
	}
	if compressed.PlanSummary == "" || compressed.PlanConfidence <= 0 {
		t.Fatalf("compression dropped plan: %+v", compressed)
	}
}

func TestEvaluatePlanCoherenceFindsGaps(t *testing.T) {
	mem := NewMemory("game", "white")
	mem.Ply = 8
	mem.Phase = "unknown"
	mem.Plan.Summary = ""
	mem.Commitments = nil
	mem.RefutationTriggers = nil

	report := EvaluatePlanCoherence(mem)
	if report.SchemaVersion != PlanCoherenceSchemaVersion || report.Score >= 0.7 || report.Status != "weak" {
		t.Fatalf("expected weak coherence, got %+v", report)
	}
	for _, want := range []string{"coherence_plan_missing", "coherence_targets_missing", "coherence_triggers_missing"} {
		if !hasStrategyAlert(report.Findings, want) {
			t.Fatalf("missing finding %s in %+v", want, report.Findings)
		}
	}
}

func TestEvaluateCandidateDiversityScoresMoveFamilies(t *testing.T) {
	candidates := []CandidateMove{
		{UCI: "e2e4", Purpose: "claim the center"},
		{UCI: "g1f3", Purpose: "develop a knight"},
		{UCI: "d1h5", Purpose: "create a check", LegalMove: testLegalMove(true, false, "")},
		{UCI: "e4d5", Purpose: "capture in the center", LegalMove: testLegalMove(false, true, "")},
	}
	report := EvaluateCandidateDiversity(candidates)
	if report.SchemaVersion != CandidateDiversitySchemaVersion || report.CandidateCount != len(candidates) {
		t.Fatalf("bad diversity boundary fields: %+v", report)
	}
	if report.Score <= 0.45 || report.Status == "narrow" {
		t.Fatalf("expected mixed/broad diversity, got %+v", report)
	}
	if len(report.Families) < 3 {
		t.Fatalf("expected several move families, got %+v", report.Families)
	}
}

func TestEvaluateCandidateDiversityParsesCustomBoardDestinations(t *testing.T) {
	candidates := []CandidateMove{
		{UCI: "a9b10"},
		{UCI: "c9b11"},
		{UCI: "d9b12"},
	}
	report := EvaluateCandidateDiversity(candidates)
	if report.Score <= 0.45 || report.Status == "narrow" {
		t.Fatalf("custom multi-digit destinations collapsed: %+v", report)
	}
}

func testLegalMove(check, capture bool, promotion string) chesscore.LegalMove {
	return chesscore.LegalMove{Capture: capture, Check: check, Promotion: promotion}
}
