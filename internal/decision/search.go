package decision

import (
	"context"
	"math"
	"sort"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const deterministicSearchName = "deterministic_mcts_material"

func applySearchScores(ctx context.Context, game *chesscore.Game, candidates []strategy.CandidateMove, mode strategy.EngineMode) (bool, string) {
	if (mode != strategy.ModeHybrid && mode != strategy.ModeCurrent) || game == nil {
		return false, ""
	}
	side := game.SideToMove()
	for i := range candidates {
		select {
		case <-ctx.Done():
			return false, deterministicSearchName
		default:
		}
		candidates[i].SearchScore = playoutSearchScore(game, candidates[i].UCI, side)
	}
	return true, deterministicSearchName
}

func playoutSearchScore(game *chesscore.Game, moveUCI string, side string) float64 {
	before := staticSideCentipawns(game, side)
	after := game.Clone()
	if _, err := after.ApplyUCI(moveUCI); err != nil {
		return -1
	}
	if score, ok := terminalSearchScore(after.Outcome(), side); ok {
		return score
	}

	replies := prioritizedSearchMoves(after.LegalMoves(), 6)
	if len(replies) == 0 {
		return clampSearchScore(float64(staticSideCentipawns(after, side)-before) / 900.0)
	}
	total := 0
	playouts := 0
	for _, reply := range replies {
		next := after.Clone()
		if _, err := next.ApplyUCI(reply.UCI); err != nil {
			continue
		}
		replyScore := bestReplyScore(next, side)
		if terminal, ok := terminalSearchScore(next.Outcome(), side); ok {
			replyScore = int(terminal * 10000)
		}
		total += replyScore
		playouts++
	}
	if playouts == 0 {
		return clampSearchScore(float64(staticSideCentipawns(after, side)-before) / 900.0)
	}
	average := total / playouts
	return clampSearchScore(float64(average-before) / 900.0)
}

func bestReplyScore(game *chesscore.Game, side string) int {
	best := staticSideCentipawns(game, side)
	for _, reply := range prioritizedSearchMoves(game.LegalMoves(), 6) {
		next := game.Clone()
		if _, err := next.ApplyUCI(reply.UCI); err != nil {
			continue
		}
		score := staticSideCentipawns(next, side)
		if terminal, ok := terminalSearchScore(next.Outcome(), side); ok {
			score = int(terminal * 10000)
		}
		if score > best {
			best = score
		}
	}
	return best
}

func prioritizedSearchMoves(moves []chesscore.LegalMove, limit int) []chesscore.LegalMove {
	out := append([]chesscore.LegalMove(nil), moves...)
	sort.SliceStable(out, func(i, j int) bool {
		return searchMovePriority(out[i]) > searchMovePriority(out[j])
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func searchMovePriority(move chesscore.LegalMove) int {
	score := 0
	if move.Check {
		score += 100
	}
	if move.Capture {
		score += 60
	}
	if move.Promotion != "" {
		score += 80
	}
	switch move.To {
	case "d4", "e4", "d5", "e5", "c4", "f4", "c5", "f5":
		score += 10
	}
	return score
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
