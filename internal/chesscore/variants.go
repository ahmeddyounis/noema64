package chesscore

import (
	"fmt"
	"strings"
)

type Variant string

const (
	VariantStandard Variant = "standard"
	VariantChess960 Variant = "chess960"
	VariantCustom   Variant = "custom"
)

const VariantStartSchemaVersion = "variant-start.v1"

const standardStartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type VariantStart struct {
	SchemaVersion   string   `json:"schema_version"`
	Variant         Variant  `json:"variant"`
	Seed            int64    `json:"seed,omitempty"`
	FEN             string   `json:"fen"`
	CastlingEnabled bool     `json:"castling_enabled"`
	Notes           []string `json:"notes,omitempty"`
}

func StandardStart(fen string) VariantStart {
	fen = strings.TrimSpace(fen)
	if fen == "" {
		fen = standardStartFEN
	}
	return VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantStandard,
		FEN:             fen,
		CastlingEnabled: true,
	}
}

func Chess960Start(seed int64) VariantStart {
	index := seed % 960
	if index < 0 {
		index += 960
	}
	backRank := chess960BackRank(int(index))
	fen := strings.ToLower(backRank) + "/pppppppp/8/8/8/8/PPPPPPPP/" + backRank + " w - - 0 1"
	return VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantChess960,
		Seed:            index,
		FEN:             fen,
		CastlingEnabled: false,
		Notes: []string{
			"Generated from a deterministic Chess960 start index.",
			"Castling is disabled until the move generator exposes Chess960 castling semantics.",
		},
	}
}

func CustomBoardStart(fen string) (VariantStart, error) {
	fen = strings.TrimSpace(fen)
	if fen == "" {
		return VariantStart{}, fmt.Errorf("custom board FEN is required")
	}
	if _, err := FromFEN(fen); err != nil {
		return VariantStart{}, err
	}
	return VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantCustom,
		FEN:             fen,
		CastlingEnabled: fenCastlingField(fen) != "-",
		Notes:           []string{"Custom board loaded from validated FEN."},
	}, nil
}

func NormalizeVariantStart(start VariantStart, fallbackFEN string) VariantStart {
	if strings.TrimSpace(start.FEN) == "" {
		start.FEN = strings.TrimSpace(fallbackFEN)
	}
	if strings.TrimSpace(start.FEN) == "" {
		start.FEN = standardStartFEN
	}
	if start.SchemaVersion == "" {
		start.SchemaVersion = VariantStartSchemaVersion
	}
	switch start.Variant {
	case VariantStandard, VariantChess960, VariantCustom:
	default:
		start.Variant = VariantCustom
	}
	if start.Variant == VariantStandard && start.FEN == standardStartFEN {
		start.CastlingEnabled = true
	}
	return start
}

func chess960BackRank(index int) string {
	board := [8]byte{}
	remaining := []int{0, 1, 2, 3, 4, 5, 6, 7}
	n := index

	darkSquares := []int{0, 2, 4, 6}
	board[darkSquares[n%4]] = 'B'
	n /= 4
	lightSquares := []int{1, 3, 5, 7}
	board[lightSquares[n%4]] = 'B'
	n /= 4
	remaining = emptySquares(board)

	qIdx := n % len(remaining)
	board[remaining[qIdx]] = 'Q'
	n /= len(remaining)
	remaining = emptySquares(board)

	knightCombos := [10][2]int{
		{0, 1}, {0, 2}, {0, 3}, {0, 4}, {1, 2},
		{1, 3}, {1, 4}, {2, 3}, {2, 4}, {3, 4},
	}
	combo := knightCombos[n%10]
	board[remaining[combo[0]]] = 'N'
	board[remaining[combo[1]]] = 'N'
	remaining = emptySquares(board)

	board[remaining[0]] = 'R'
	board[remaining[1]] = 'K'
	board[remaining[2]] = 'R'
	return string(board[:])
}

func emptySquares(board [8]byte) []int {
	out := make([]int, 0, 8)
	for i, piece := range board {
		if piece == 0 {
			out = append(out, i)
		}
	}
	return out
}

func fenCastlingField(fen string) string {
	fields := strings.Fields(fen)
	if len(fields) < 3 {
		return ""
	}
	return fields[2]
}
