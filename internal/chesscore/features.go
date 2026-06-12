package chesscore

import chess "github.com/corentings/chess/v2"

func (g *Game) Features() FeatureSummary {
	legal := g.LegalMoves()
	features := FeatureSummary{
		MaterialBalance: g.materialBalance(),
		Phase:           g.phase(),
		SideToMove:      g.SideToMove(),
		LegalMoveCount:  len(legal),
	}
	for _, mv := range legal {
		if mv.Capture {
			features.Captures = append(features.Captures, mv.UCI)
		}
		if mv.Check {
			features.Checks = append(features.Checks, mv.UCI)
		}
	}
	return features
}

func (g *Game) materialBalance() int {
	score := 0
	for _, piece := range g.g.Position().Board().SquareMap() {
		value := pieceValue(piece.Type())
		if piece.Color() == chess.White {
			score += value
		}
		if piece.Color() == chess.Black {
			score -= value
		}
	}
	return score
}

func (g *Game) phase() string {
	pieceCount := 0
	queenCount := 0
	for _, piece := range g.g.Position().Board().SquareMap() {
		if piece == chess.NoPiece {
			continue
		}
		pieceCount++
		if piece.Type() == chess.Queen {
			queenCount++
		}
	}
	switch {
	case pieceCount <= 10:
		return "endgame"
	case queenCount < 2 || g.Ply() > 20:
		return "middlegame"
	default:
		return "opening"
	}
}

func pieceValue(t chess.PieceType) int {
	switch t {
	case chess.Pawn:
		return 100
	case chess.Knight, chess.Bishop:
		return 320
	case chess.Rook:
		return 500
	case chess.Queen:
		return 900
	default:
		return 0
	}
}
