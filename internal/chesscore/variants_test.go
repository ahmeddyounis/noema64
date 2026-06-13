package chesscore

import (
	"strings"
	"testing"
)

func TestChess960StartGeneratesValidBackRank(t *testing.T) {
	for _, seed := range []int64{0, 1, 42, 959, 960, -1} {
		start := Chess960Start(seed)
		game, err := FromVariantStart(start)
		if err != nil {
			t.Fatalf("seed %d produced invalid FEN %q: %v", seed, start.FEN, err)
		}
		if start.SchemaVersion != VariantStartSchemaVersion || start.Variant != VariantChess960 {
			t.Fatalf("seed %d produced incomplete metadata: %+v", seed, start)
		}
		if !start.CastlingEnabled || start.CastlingMode != CastlingModeChess960External || start.CastlingRights != "KQkq" {
			t.Fatalf("seed %d missing Chess960 castling metadata: %+v", seed, start)
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

func TestChess960ExternalCastlingIsLegalAndPlayable(t *testing.T) {
	start := VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantChess960,
		FEN:             "r3k2r/8/8/8/8/8/8/RK5R w - - 0 1",
		RuleSet:         "chess960",
		CastlingEnabled: true,
		CastlingMode:    CastlingModeChess960External,
		CastlingRights:  "KQkq",
	}
	game, err := FromVariantStart(start)
	if err != nil {
		t.Fatalf("variant start: %v", err)
	}
	if !game.IsLegalUCI("b1h1") || !game.IsLegalUCI("b1a1") {
		t.Fatalf("missing Chess960 castling moves: %+v", game.LegalMoves())
	}
	rec, err := game.ApplyUCI("b1h1")
	if err != nil {
		t.Fatalf("apply Chess960 castle: %v", err)
	}
	if rec.SAN != "O-O" || game.Snapshot().Board["g1"] != "♔" || game.Snapshot().Board["f1"] != "♖" {
		t.Fatalf("bad Chess960 castle record=%+v board=%+v", rec, game.Snapshot().Board)
	}
	if game.IsLegalUCI("g1h1") {
		t.Fatalf("castling remained legal after king castled: %+v", game.LegalMoves())
	}
	if pgn := game.PGN(); !strings.Contains(pgn, "1. O-O") {
		t.Fatalf("Chess960 castling PGN missing castle: %s", pgn)
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

func TestCustomBoardDefinitionPersistsRuleMetadata(t *testing.T) {
	start, err := CustomBoardStartFromDefinition(CustomBoardDefinition{
		SchemaVersion: CustomBoardDefinitionSchemaVersion,
		ID:            "archbishop-lab",
		Name:          "Archbishop Lab",
		InitialFEN:    "8/8/8/8/8/8/8/R3K2R w KQ - 0 1",
		RuleSet:       "custom-piece-lab",
		BoardWidth:    8,
		BoardHeight:   8,
		PieceRules: []CustomPieceRule{{
			Symbol: "A",
			Name:   "Archbishop",
			Move:   "bishop+knight",
		}},
	})
	if err != nil {
		t.Fatalf("custom board definition: %v", err)
	}
	if start.BoardDefinition == nil || start.RuleSet != "custom-piece-lab" || len(start.UnsupportedRules) == 0 {
		t.Fatalf("custom rule metadata was not preserved: %+v", start)
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
