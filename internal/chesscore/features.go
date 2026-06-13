package chesscore

import (
	"fmt"
	"sort"

	chess "github.com/corentings/chess/v2"
)

func (g *Game) Features() FeatureSummary {
	if g.custom != nil {
		return g.custom.features(g.Ply())
	}
	legal := g.LegalMoves()
	board := g.g.Position().Board().SquareMap()
	side := g.g.Position().Turn()
	kingSq, hasKing := kingSquare(board, side)
	kingAttackers := []string{}
	if hasKing {
		kingAttackers = attackingPieces(board, kingSq, side.Other())
	}
	features := FeatureSummary{
		MaterialBalance: g.materialBalance(),
		Phase:           g.phase(),
		SideToMove:      g.SideToMove(),
		InCheck:         len(kingAttackers) > 0,
		LegalMoveCount:  len(legal),
	}
	for _, mv := range legal {
		if mv.Capture {
			features.Captures = append(features.Captures, mv.UCI)
			features.Threats = append(features.Threats, fmt.Sprintf("%s captures on %s", mv.UCI, mv.To))
		}
		if mv.Check {
			features.Checks = append(features.Checks, mv.UCI)
			features.Threats = append(features.Threats, fmt.Sprintf("%s gives check", mv.UCI))
		}
	}
	features.PinnedPieces = pinnedPieces(board, side, kingSq, hasKing)
	features.HangingPieces = hangingPieces(board, side)
	features.KingSafety = kingSafetyFlags(kingAttackers, legal, kingSq, hasKing)
	sort.Strings(features.Captures)
	sort.Strings(features.Checks)
	sort.Strings(features.Threats)
	return features
}

func kingSquare(board map[chess.Square]chess.Piece, color chess.Color) (chess.Square, bool) {
	for sq, piece := range board {
		if piece.Type() == chess.King && piece.Color() == color {
			return sq, true
		}
	}
	return chess.NoSquare, false
}

func pinnedPieces(board map[chess.Square]chess.Piece, side chess.Color, kingSq chess.Square, hasKing bool) []string {
	if !hasKing {
		return nil
	}
	out := []string{}
	for sq, piece := range board {
		if piece == chess.NoPiece || piece.Color() != side || piece.Type() == chess.King {
			continue
		}
		without := copyBoard(board)
		delete(without, sq)
		if len(attackingPieces(without, kingSq, side.Other())) > 0 {
			out = append(out, fmt.Sprintf("%s:%s pinned to king", sq, featurePieceName(piece)))
		}
	}
	sort.Strings(out)
	return out
}

func hangingPieces(board map[chess.Square]chess.Piece, side chess.Color) []string {
	out := []string{}
	for sq, piece := range board {
		if piece == chess.NoPiece || piece.Color() != side || piece.Type() == chess.King {
			continue
		}
		attackers := attackingPieces(board, sq, side.Other())
		if len(attackers) == 0 {
			continue
		}
		defenders := attackingPieces(board, sq, side)
		if len(defenders) == 0 {
			out = append(out, fmt.Sprintf("%s:%s attacked by %s", sq, featurePieceName(piece), attackers[0]))
		}
	}
	sort.Strings(out)
	return out
}

func kingSafetyFlags(attackers []string, legal []LegalMove, kingSq chess.Square, hasKing bool) []string {
	flags := []string{}
	if len(attackers) > 0 {
		flags = append(flags, "in_check")
		for _, attacker := range attackers {
			flags = append(flags, "king_attacked_by:"+attacker)
		}
	}
	if hasKing {
		kingMoves := 0
		for _, mv := range legal {
			if mv.From == kingSq.String() {
				kingMoves++
			}
		}
		if len(attackers) > 0 && kingMoves <= 1 {
			flags = append(flags, fmt.Sprintf("limited_king_mobility:%d", kingMoves))
		}
	}
	sort.Strings(flags)
	return flags
}

func attackingPieces(board map[chess.Square]chess.Piece, target chess.Square, color chess.Color) []string {
	out := []string{}
	for from, piece := range board {
		if piece == chess.NoPiece || piece.Color() != color {
			continue
		}
		if pieceAttacksSquare(board, from, piece, target) {
			out = append(out, fmt.Sprintf("%s:%s", from, featurePieceName(piece)))
		}
	}
	sort.Strings(out)
	return out
}

func pieceAttacksSquare(board map[chess.Square]chess.Piece, from chess.Square, piece chess.Piece, target chess.Square) bool {
	if from == target {
		return false
	}
	df := int(target.File()) - int(from.File())
	dr := int(target.Rank()) - int(from.Rank())
	absDF := absInt(df)
	absDR := absInt(dr)
	switch piece.Type() {
	case chess.Pawn:
		if piece.Color() == chess.White {
			return dr == 1 && absDF == 1
		}
		return dr == -1 && absDF == 1
	case chess.Knight:
		return (absDF == 1 && absDR == 2) || (absDF == 2 && absDR == 1)
	case chess.Bishop:
		return absDF == absDR && clearPath(board, from, target, signInt(df), signInt(dr))
	case chess.Rook:
		return (df == 0 || dr == 0) && clearPath(board, from, target, signInt(df), signInt(dr))
	case chess.Queen:
		diagonal := absDF == absDR
		straight := df == 0 || dr == 0
		return (diagonal || straight) && clearPath(board, from, target, signInt(df), signInt(dr))
	case chess.King:
		return absDF <= 1 && absDR <= 1
	default:
		return false
	}
}

func clearPath(board map[chess.Square]chess.Piece, from chess.Square, target chess.Square, stepFile int, stepRank int) bool {
	if stepFile == 0 && stepRank == 0 {
		return false
	}
	file := int(from.File()) + stepFile
	rank := int(from.Rank()) + stepRank
	targetFile := int(target.File())
	targetRank := int(target.Rank())
	for file != targetFile || rank != targetRank {
		sq := chess.NewSquare(chess.File(file), chess.Rank(rank))
		if piece, ok := board[sq]; ok && piece != chess.NoPiece {
			return false
		}
		file += stepFile
		rank += stepRank
	}
	return true
}

func copyBoard(board map[chess.Square]chess.Piece) map[chess.Square]chess.Piece {
	out := make(map[chess.Square]chess.Piece, len(board))
	for sq, piece := range board {
		out[sq] = piece
	}
	return out
}

func featurePieceName(piece chess.Piece) string {
	color := "unknown"
	switch piece.Color() {
	case chess.White:
		color = "white"
	case chess.Black:
		color = "black"
	}
	return color + "_" + featurePieceTypeName(piece.Type())
}

func featurePieceTypeName(pieceType chess.PieceType) string {
	switch pieceType {
	case chess.King:
		return "king"
	case chess.Queen:
		return "queen"
	case chess.Rook:
		return "rook"
	case chess.Bishop:
		return "bishop"
	case chess.Knight:
		return "knight"
	case chess.Pawn:
		return "pawn"
	default:
		return "piece"
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func signInt(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func (g *Game) materialBalance() int {
	if g.custom != nil {
		return g.custom.materialBalance()
	}
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
	if g.custom != nil {
		return g.custom.phase(g.Ply())
	}
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
