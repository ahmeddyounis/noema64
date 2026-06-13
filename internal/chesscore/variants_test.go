package chesscore

import "testing"

func TestChess960StartGeneratesValidBackRank(t *testing.T) {
	for _, seed := range []int64{0, 1, 42, 959, 960, -1} {
		start := Chess960Start(seed)
		game, err := FromFEN(start.FEN)
		if err != nil {
			t.Fatalf("seed %d produced invalid FEN %q: %v", seed, start.FEN, err)
		}
		if start.SchemaVersion != VariantStartSchemaVersion || start.Variant != VariantChess960 {
			t.Fatalf("seed %d produced incomplete metadata: %+v", seed, start)
		}
		if start.CastlingEnabled {
			t.Fatalf("seed %d enabled castling despite Chess960 castling not being supported", seed)
		}
		if len(game.LegalMoves()) == 0 {
			t.Fatalf("seed %d produced no legal opening moves", seed)
		}

		rank := backRankFromFEN(start.FEN)
		if len(rank) != 8 {
			t.Fatalf("rank = %q, want 8 files", rank)
		}
		if !bishopsOnOppositeColors(rank) {
			t.Fatalf("rank %q has bishops on same color", rank)
		}
		if !kingBetweenRooks(rank) {
			t.Fatalf("rank %q does not place king between rooks", rank)
		}
	}
}

func TestCustomBoardStartValidatesFEN(t *testing.T) {
	start, err := CustomBoardStart("8/P7/8/8/8/8/8/4k2K w - - 0 1")
	if err != nil {
		t.Fatalf("custom board: %v", err)
	}
	if start.Variant != VariantCustom || start.FEN == "" || start.CastlingEnabled {
		t.Fatalf("unexpected custom metadata: %+v", start)
	}
	if _, err := CustomBoardStart("not a fen"); err == nil {
		t.Fatal("expected invalid custom FEN to fail")
	}
}

func backRankFromFEN(fen string) string {
	for i, r := range fen {
		if r == '/' {
			return fen[:i]
		}
	}
	return fen
}

func bishopsOnOppositeColors(rank string) bool {
	colors := []int{}
	for i, piece := range rank {
		if piece == 'b' {
			colors = append(colors, i%2)
		}
	}
	return len(colors) == 2 && colors[0] != colors[1]
}

func kingBetweenRooks(rank string) bool {
	rookBefore := false
	kingSeen := false
	for _, piece := range rank {
		switch piece {
		case 'r':
			if kingSeen {
				return rookBefore
			}
			rookBefore = true
		case 'k':
			kingSeen = true
		}
	}
	return false
}
