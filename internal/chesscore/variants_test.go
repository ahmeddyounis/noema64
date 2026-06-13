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
		InitialFEN:    "4k3/8/8/8/3A4/8/8/4K3 w - - 0 1",
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
	if start.BoardDefinition == nil || start.RuleSet != "custom-piece-lab" || len(start.UnsupportedRules) != 0 {
		t.Fatalf("custom rule metadata was not preserved: %+v", start)
	}
	game, err := FromVariantStart(start)
	if err != nil {
		t.Fatalf("custom variant game: %v", err)
	}
	if !game.IsLegalUCI("d4e6") || !game.IsLegalUCI("d4h8") {
		t.Fatalf("custom archbishop moves missing: %+v", game.LegalMoves())
	}
	rec, err := game.ApplyUCI("d4e6")
	if err != nil {
		t.Fatalf("apply custom archbishop move: %v", err)
	}
	if rec.SAN != "Ae6" || game.BoardMap()["e6"] != "A" || !strings.Contains(game.PGN(), "1. Ae6") {
		t.Fatalf("custom move was not recorded correctly: rec=%+v board=%+v pgn=%s", rec, game.BoardMap(), game.PGN())
	}
	if game.Features().MaterialBalance <= 0 {
		t.Fatalf("custom material balance did not value archbishop: %+v", game.Features())
	}
}

func TestCustomBoardDefinitionRejectsUndefinedPiecesAndMissingRoyals(t *testing.T) {
	base := CustomBoardDefinition{
		SchemaVersion: CustomBoardDefinitionSchemaVersion,
		ID:            "bad-custom",
		Name:          "Bad Custom",
		InitialFEN:    "4k3/8/8/8/3A4/8/8/4K3 w - - 0 1",
		RuleSet:       "custom-piece-lab",
		BoardWidth:    8,
		BoardHeight:   8,
	}
	if _, err := CustomBoardStartFromDefinition(base); err == nil || !strings.Contains(err.Error(), "requires a piece_rules entry") {
		t.Fatalf("expected undefined custom piece failure, got %v", err)
	}

	base.InitialFEN = "8/8/8/8/3A4/8/8/4K3 w - - 0 1"
	base.PieceRules = []CustomPieceRule{{Symbol: "A", Name: "Archbishop", Move: "bishop+knight"}}
	if _, err := CustomBoardStartFromDefinition(base); err == nil || !strings.Contains(err.Error(), "royal piece") {
		t.Fatalf("expected missing royal failure, got %v", err)
	}
}

func TestCustomBoardDefinitionSupportsCustomRoyalPieces(t *testing.T) {
	start, err := CustomBoardStartFromDefinition(CustomBoardDefinition{
		SchemaVersion: CustomBoardDefinitionSchemaVersion,
		ID:            "duke-lab",
		Name:          "Duke Lab",
		InitialFEN:    "4x3/8/8/8/8/8/8/4X3 w - - 0 1",
		RuleSet:       "custom-royal-lab",
		BoardWidth:    8,
		BoardHeight:   8,
		PieceRules: []CustomPieceRule{{
			Symbol: "X",
			Name:   "Duke",
			Move:   "king",
			Royal:  true,
		}},
	})
	if err != nil {
		t.Fatalf("custom royal board: %v", err)
	}
	game, err := FromVariantStart(start)
	if err != nil {
		t.Fatalf("custom royal game: %v", err)
	}
	if !game.IsLegalUCI("e1d1") || game.Outcome().Status != "ongoing" {
		t.Fatalf("custom royal game not playable: outcome=%+v moves=%+v", game.Outcome(), game.LegalMoves())
	}
}

func TestCustomBoardDefinitionRejectsInvalidRuleMovementAndPromotions(t *testing.T) {
	base := CustomBoardDefinition{
		SchemaVersion: CustomBoardDefinitionSchemaVersion,
		ID:            "bad-rules",
		Name:          "Bad Rules",
		InitialFEN:    "4k3/8/8/8/3A4/8/8/4K3 w - - 0 1",
		RuleSet:       "custom-piece-lab",
		BoardWidth:    8,
		BoardHeight:   8,
		PieceRules: []CustomPieceRule{{
			Symbol: "A",
			Name:   "Archbishop",
			Move:   "teleport",
		}},
	}
	if _, err := CustomBoardStartFromDefinition(base); err == nil || !strings.Contains(err.Error(), "unsupported move token") {
		t.Fatalf("expected invalid movement token failure, got %v", err)
	}

	base.PieceRules = []CustomPieceRule{{
		Symbol:     "A",
		Name:       "Archbishop",
		Move:       "bishop+knight",
		PromotesTo: []string{"Z"},
	}}
	if _, err := CustomBoardStartFromDefinition(base); err == nil || !strings.Contains(err.Error(), "requires a piece rule") {
		t.Fatalf("expected undefined promotion target failure, got %v", err)
	}

	base.PieceRules[0].PromotesTo = []string{"K"}
	if _, err := CustomBoardStartFromDefinition(base); err == nil || !strings.Contains(err.Error(), "cannot be royal") {
		t.Fatalf("expected royal promotion target failure, got %v", err)
	}
}

func TestCustomPawnRuleCanPromoteToCustomPiece(t *testing.T) {
	start, err := CustomBoardStartFromDefinition(CustomBoardDefinition{
		SchemaVersion: CustomBoardDefinitionSchemaVersion,
		ID:            "serpent-pawn-lab",
		Name:          "Serpent Pawn Lab",
		InitialFEN:    "4k3/1S6/8/8/8/8/8/4K3 w - - 0 1",
		RuleSet:       "custom-pawn-lab",
		BoardWidth:    8,
		BoardHeight:   8,
		PieceRules: []CustomPieceRule{
			{
				Symbol:     "S",
				Name:       "Serpent",
				Move:       "pawn",
				PromotesTo: []string{"A", "A"},
			},
			{
				Symbol: "A",
				Name:   "Archbishop",
				Move:   "bishop+knight",
			},
		},
	})
	if err != nil {
		t.Fatalf("custom pawn board: %v", err)
	}
	if got := start.BoardDefinition.PieceRules[0].PromotesTo; len(got) != 1 || got[0] != "A" {
		t.Fatalf("promotions were not normalized/deduped: %+v", got)
	}
	game, err := FromVariantStart(start)
	if err != nil {
		t.Fatalf("custom pawn game: %v", err)
	}
	if game.IsLegalUCI("b7a7") {
		t.Fatalf("custom pawn fell back to king movement: %+v", game.LegalMoves())
	}
	if !game.IsLegalUCI("b7b8a") {
		t.Fatalf("custom pawn promotion missing: %+v", game.LegalMoves())
	}
	rec, err := game.ApplyUCI("b7b8a")
	if err != nil {
		t.Fatalf("apply custom pawn promotion: %v", err)
	}
	if rec.SAN != "b8=A" || game.BoardMap()["b8"] != "A" {
		t.Fatalf("custom pawn promotion record=%+v board=%+v", rec, game.BoardMap())
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
