package decision

import (
	"context"
	"math"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const deterministicSearchName = "deterministic_2ply_material"

func applySearchScores(ctx context.Context, game *chesscore.Game, candidates []strategy.CandidateMove, mode strategy.EngineMode) (bool, string) {
	if mode != strategy.ModeHybrid || game == nil {
		return false, ""
	}
	side := game.SideToMove()
	for i := range candidates {
		select {
		case <-ctx.Done():
			return false, deterministicSearchName
		default:
		}
		candidates[i].SearchScore = shallowSearchScore(game, candidates[i].UCI, side)
	}
	return true, deterministicSearchName
}

func shallowSearchScore(game *chesscore.Game, moveUCI string, side string) float64 {
	before := staticSideCentipawns(game, side)
	after := game.Clone()
	if _, err := after.ApplyUCI(moveUCI); err != nil {
		return -1
	}
	if score, ok := terminalSearchScore(after.Outcome(), side); ok {
		return score
	}

	worst := staticSideCentipawns(after, side)
	replies := after.LegalMoves()
	for _, reply := range replies {
		next := after.Clone()
		if _, err := next.ApplyUCI(reply.UCI); err != nil {
			continue
		}
		replyScore := staticSideCentipawns(next, side)
		if terminal, ok := terminalSearchScore(next.Outcome(), side); ok {
			replyScore = int(terminal * 10000)
		}
		if replyScore < worst {
			worst = replyScore
		}
	}
	return clampSearchScore(float64(worst-before) / 900.0)
}

func staticSideCentipawns(game *chesscore.Game, side string) int {
	score := game.Features().MaterialBalance
	if side == "black" {
		return -score
	}
	return score
}

func terminalSearchScore(outcome chesscore.Outcome, side string) (float64, bool) {
	switch outcome.Status {
	case "checkmate", "resignation":
		if outcome.Winner == side {
			return 1, true
		}
		if outcome.Winner != "" {
			return -1, true
		}
	case "draw":
		return 0, true
	}
	return 0, false
}

func clampSearchScore(score float64) float64 {
	if math.IsNaN(score) {
		return 0
	}
	if score > 1 {
		return 1
	}
	if score < -1 {
		return -1
	}
	return score
}
