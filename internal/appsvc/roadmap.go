package appsvc

import (
	"context"
	"time"

	"github.com/ahmedyounis/noema64/internal/analysis"
	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const studyDashboardSchemaVersion = "study-dashboard.v1"

type StudyDashboard struct {
	SchemaVersion      string                            `json:"schema_version"`
	GeneratedAt        string                            `json:"generated_at"`
	GameID             string                            `json:"game_id"`
	Ply                int                               `json:"ply"`
	Variant            chesscore.VariantStart            `json:"variant"`
	Memory             strategy.CompressedMemory         `json:"memory"`
	Coherence          strategy.PlanCoherenceReport      `json:"coherence"`
	CandidateDiversity strategy.CandidateDiversityReport `json:"candidate_diversity"`
	MultiAgent         analysis.MultiAgentReview         `json:"multi_agent"`
	Lesson             StudyLesson                       `json:"lesson"`
	Puzzle             StudyPuzzle                       `json:"puzzle"`
	Heatmap            []StudyHeat                       `json:"heatmap,omitempty"`
	Timeline           []StrategyTimelineItem            `json:"timeline,omitempty"`
}

type StudyLesson struct {
	Title string   `json:"title"`
	Focus string   `json:"focus"`
	Steps []string `json:"steps"`
}

type StudyPuzzle struct {
	FEN            string   `json:"fen"`
	Goal           string   `json:"goal"`
	CandidateMoves []string `json:"candidate_moves,omitempty"`
	Solution       string   `json:"solution,omitempty"`
}

type StudyHeat struct {
	Square string  `json:"square"`
	Move   string  `json:"move"`
	Weight float64 `json:"weight"`
	Label  string  `json:"label"`
}

type StrategyTimelineItem struct {
	Ply     int    `json:"ply"`
	Move    string `json:"move,omitempty"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

func (a *Application) CompressedStrategyMemory(maxItems int) (strategy.CompressedMemory, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.CompressedMemory{}, appErr("ERR_GAME_STATE", err, true)
	}
	return strategy.CompressMemory(state.StrategyMemory, maxItems), nil
}

func (a *Application) PlanCoherence() (strategy.PlanCoherenceReport, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.PlanCoherenceReport{}, appErr("ERR_GAME_STATE", err, true)
	}
	return strategy.EvaluatePlanCoherence(state.StrategyMemory), nil
}

func (a *Application) CandidateDiversity() (strategy.CandidateDiversityReport, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.CandidateDiversityReport{}, appErr("ERR_GAME_STATE", err, true)
	}
	if state.LastDecision == nil {
		return strategy.EvaluateCandidateDiversity(nil), nil
	}
	return strategy.EvaluateCandidateDiversity(state.LastDecision.CandidateMoves), nil
}

func (a *Application) MultiAgentAnalysis() (analysis.MultiAgentReview, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return analysis.MultiAgentReview{}, appErr("ERR_GAME_STATE", err, true)
	}
	return analysis.ReviewDecision(state.LastDecision), nil
}

func (a *Application) StudyDashboard() (StudyDashboard, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return StudyDashboard{}, appErr("ERR_GAME_STATE", err, true)
	}
	return studyDashboardFromState(state), nil
}

func (a *Application) UpdateStrategyMemory(memory strategy.StrategyMemory) (*engine.GameState, error) {
	state, err := a.engine.UpdateStrategyMemory(context.Background(), memory)
	if err != nil {
		return state, appErr("ERR_STRATEGY_MEMORY", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func studyDashboardFromState(state *engine.GameState) StudyDashboard {
	decision := state.LastDecision
	candidates := []strategy.CandidateMove{}
	if decision != nil {
		candidates = decision.CandidateMoves
	}
	return StudyDashboard{
		SchemaVersion:      studyDashboardSchemaVersion,
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		GameID:             state.Snapshot.GameID,
		Ply:                state.Snapshot.Ply,
		Variant:            state.Variant,
		Memory:             strategy.CompressMemory(state.StrategyMemory, 5),
		Coherence:          strategy.EvaluatePlanCoherence(state.StrategyMemory),
		CandidateDiversity: strategy.EvaluateCandidateDiversity(candidates),
		MultiAgent:         analysis.ReviewDecision(decision),
		Lesson:             lessonForState(state),
		Puzzle:             puzzleForState(state),
		Heatmap:            heatmapForDecision(decision),
		Timeline:           timelineForState(state),
	}
}

func lessonForState(state *engine.GameState) StudyLesson {
	metrics := state.StrategyMetrics
	if state.LastDecision == nil {
		return StudyLesson{
			Title: "First Plan",
			Focus: "Create a strategy memory baseline before studying variations.",
			Steps: []string{
				"Run analysis from the current position.",
				"Compare candidate purposes against legal moves.",
				"Confirm the plan has targets, commitments, and refutation triggers.",
			},
		}
	}
	if metrics.AlertLevel == "high" || metrics.Quality < 0.6 {
		return StudyLesson{
			Title: "Plan Repair",
			Focus: "Resolve weak or drifting strategy memory.",
			Steps: []string{
				"Read the strategist and critic findings.",
				"Edit the plan summary or commitments if they no longer match the board.",
				"Run hybrid analysis to refresh verifier and search evidence.",
			},
		}
	}
	return StudyLesson{
		Title: "Candidate Comparison",
		Focus: "Understand why the selected move outranked alternatives.",
		Steps: []string{
			"Review the candidate heatmap and final scores.",
			"Check whether warnings or rejected candidates explain the ranking.",
			"Use Why Not on a candidate move to compare it directly with the selection.",
		},
	}
}

func puzzleForState(state *engine.GameState) StudyPuzzle {
	puzzle := StudyPuzzle{
		FEN:  state.Snapshot.FEN,
		Goal: "Choose the move Noema64 selected and explain the strategic reason.",
	}
	if state.LastDecision == nil {
		puzzle.Goal = "Run analysis, then solve the generated candidate puzzle."
		return puzzle
	}
	puzzle.Solution = state.LastDecision.SelectedMove.UCI
	for _, candidate := range state.LastDecision.CandidateMoves {
		puzzle.CandidateMoves = append(puzzle.CandidateMoves, candidate.UCI)
	}
	return puzzle
}

func heatmapForDecision(decision *decision.MoveDecision) []StudyHeat {
	if decision == nil {
		return nil
	}
	out := []StudyHeat{}
	for _, candidate := range decision.CandidateMoves {
		square := ""
		if len(candidate.UCI) >= 4 {
			square = candidate.UCI[2:4]
		}
		out = append(out, StudyHeat{
			Square: square,
			Move:   candidate.UCI,
			Weight: candidate.FinalScore,
			Label:  candidate.VerifierScore.Status,
		})
	}
	return out
}

func timelineForState(state *engine.GameState) []StrategyTimelineItem {
	out := []StrategyTimelineItem{{
		Ply:     0,
		Status:  "start",
		Summary: "Game initialized.",
	}}
	for _, move := range state.Snapshot.MoveHistory {
		out = append(out, StrategyTimelineItem{
			Ply:     move.Ply,
			Move:    move.UCI,
			Status:  "played",
			Summary: move.Comment,
		})
	}
	if state.StrategyMemory.LastUpdate.Summary != "" {
		out = append(out, StrategyTimelineItem{
			Ply:     state.StrategyMemory.Ply,
			Move:    state.StrategyMemory.LastUpdate.MovePlayed,
			Status:  state.StrategyMemory.Plan.Status,
			Summary: state.StrategyMemory.LastUpdate.Summary,
		})
	}
	return out
}
