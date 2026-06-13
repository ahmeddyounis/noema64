package chesscore

import (
	"strings"
	"testing"
)

func TestStartPositionLegalMovesAndApply(t *testing.T) {
	game := NewGame()
	moves := game.LegalMoves()
	if len(moves) != 20 {
		t.Fatalf("start position legal moves = %d, want 20", len(moves))
	}
	if _, err := game.ApplyUCI("e2e4"); err != nil {
		t.Fatalf("apply e2e4: %v", err)
	}
	if game.SideToMove() != "black" {
		t.Fatalf("side to move = %s, want black", game.SideToMove())
	}
	if _, err := game.ApplyUCI("e2e5"); err == nil {
		t.Fatal("expected illegal move to fail")
	}
}

func TestSplitUCIMoveSquaresSupportsMultiDigitRanks(t *testing.T) {
	tests := []struct {
		uci  string
		from string
		to   string
		ok   bool
	}{
		{uci: "e2e4", from: "e2", to: "e4", ok: true},
		{uci: "a7a8q", from: "a7", to: "a8", ok: true},
		{uci: "b9b10a", from: "b9", to: "b10", ok: true},
		{uci: "a10z99", from: "a10", to: "z99", ok: true},
		{uci: "a10", ok: false},
		{uci: "10a11", ok: false},
		{uci: "a10b", ok: false},
	}
	for _, tt := range tests {
		from, to, ok := SplitUCIMoveSquares(tt.uci)
		if ok != tt.ok || from != tt.from || to != tt.to {
			t.Fatalf("SplitUCIMoveSquares(%q) = (%q, %q, %t), want (%q, %q, %t)", tt.uci, from, to, ok, tt.from, tt.to, tt.ok)
		}
	}
}

func TestStartPositionPerft(t *testing.T) {
	game := NewGame()
	tests := []struct {
		depth int
		nodes int
	}{
		{depth: 1, nodes: 20},
		{depth: 2, nodes: 400},
		{depth: 3, nodes: 8902},
	}
	for _, tt := range tests {
		if nodes := perft(game, tt.depth); nodes != tt.nodes {
			t.Fatalf("perft(%d) = %d, want %d", tt.depth, nodes, tt.nodes)
		}
	}
}

func TestFENAndPGN(t *testing.T) {
	game, err := FromFEN("8/P7/8/8/8/8/8/4k2K w - - 0 1")
	if err != nil {
		t.Fatalf("fen: %v", err)
	}
	foundPromotion := false
	foundUnderpromotion := false
	for _, mv := range game.LegalMoves() {
		if mv.UCI == "a7a8q" {
			foundPromotion = true
		}
		if mv.UCI == "a7a8n" && mv.Promotion == "n" {
			foundUnderpromotion = true
		}
	}
	if !foundPromotion {
		t.Fatal("expected queen promotion legal move")
	}
	if !foundUnderpromotion {
		t.Fatal("expected knight underpromotion legal move")
	}

	pgn := "1. e4 e5 2. Nf3 Nc6 *"
	loaded, err := FromPGN(strings.NewReader(pgn))
	if err != nil {
		t.Fatalf("pgn import: %v", err)
	}
	if len(loaded.MoveHistory()) != 4 {
		t.Fatalf("history length = %d, want 4", len(loaded.MoveHistory()))
	}
	if !strings.Contains(loaded.PGN(), "Nf3") {
		t.Fatalf("exported PGN missing Nf3: %s", loaded.PGN())
	}
}

func TestSpecialMoveLegality(t *testing.T) {
	castling, err := FromFEN("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	if err != nil {
		t.Fatalf("castling fen: %v", err)
	}
	if !hasLegalMove(castling.LegalMoves(), "e1g1") || !hasLegalMove(castling.LegalMoves(), "e1c1") {
		t.Fatalf("expected both white castling moves, got %+v", castling.LegalMoves())
	}

	enPassant, err := FromFEN("8/8/8/3pP3/8/8/8/4K2k w - d6 0 1")
	if err != nil {
		t.Fatalf("en passant fen: %v", err)
	}
	epMove, ok := findLegalMove(enPassant.LegalMoves(), "e5d6")
	if !ok || !epMove.Capture {
		t.Fatalf("expected e5d6 en passant capture, got %+v", enPassant.LegalMoves())
	}
	if _, err := enPassant.ApplyUCI("e5d6"); err != nil {
		t.Fatalf("apply en passant: %v", err)
	}
	if enPassant.BoardMap()["d5"] != "" {
		t.Fatalf("en passant left captured pawn on d5: %+v", enPassant.BoardMap())
	}
}

func TestDrawOutcomesFromFEN(t *testing.T) {
	tests := []struct {
		name string
		fen  string
	}{
		{name: "stalemate", fen: "7k/5K2/6Q1/8/8/8/8/8 b - - 0 1"},
		{name: "insufficient material", fen: "8/8/8/8/8/8/8/K6k w - - 0 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, err := FromFEN(tt.fen)
			if err != nil {
				t.Fatalf("fen: %v", err)
			}
			if outcome := game.Outcome(); outcome.Status != "draw" {
				t.Fatalf("outcome = %+v, want draw", outcome)
			}
			if len(game.LegalMoves()) != 0 {
				t.Fatalf("drawn terminal position kept legal moves: %+v", game.LegalMoves())
			}
		})
	}
}

func TestUndoTruncatesHistoryAndAllowsNewLine(t *testing.T) {
	game := NewGame()
	for _, move := range []string{"e2e4", "e7e5", "g1f3"} {
		if _, err := game.ApplyUCI(move); err != nil {
			t.Fatalf("apply %s: %v", move, err)
		}
	}
	if undone := game.Undo(2); undone != 2 {
		t.Fatalf("undone = %d, want 2", undone)
	}
	if len(game.MoveHistory()) != 1 {
		t.Fatalf("history length = %d, want 1", len(game.MoveHistory()))
	}
	if game.SideToMove() != "black" {
		t.Fatalf("side = %s, want black", game.SideToMove())
	}
	if _, err := game.ApplyUCI("c7c5"); err != nil {
		t.Fatalf("apply new line: %v", err)
	}
	history := game.MoveHistory()
	if len(history) != 2 || history[1].UCI != "c7c5" {
		t.Fatalf("unexpected history after new line: %+v", history)
	}
}

func TestResignSetsTerminalOutcome(t *testing.T) {
	game := NewGame()
	if err := game.Resign("white"); err != nil {
		t.Fatalf("resign: %v", err)
	}
	outcome := game.Outcome()
	if outcome.Status != "resignation" || outcome.Winner != "black" {
		t.Fatalf("outcome = %+v, want black resignation win", outcome)
	}
	if len(game.LegalMoves()) != 0 {
		t.Fatalf("resigned game kept legal moves")
	}
	if game.Ply() != 0 || len(game.MoveHistory()) != 0 {
		t.Fatalf("resignation created move history: ply=%d history=%+v", game.Ply(), game.MoveHistory())
	}
	if pgn := strings.TrimSpace(game.PGN()); pgn != "0-1" {
		t.Fatalf("resignation PGN = %q, want 0-1", pgn)
	}
	if _, err := game.ApplyUCI("e2e4"); err == nil {
		t.Fatal("expected move after resignation to fail")
	}
	if err := game.Resign("black"); err == nil {
		t.Fatal("expected second resignation to fail")
	}
}

func perft(game *Game, depth int) int {
	if depth == 0 {
		return 1
	}
	nodes := 0
	for _, move := range game.LegalMoves() {
		next := game.Clone()
		if _, err := next.ApplyUCI(move.UCI); err != nil {
			panic(err)
		}
		nodes += perft(next, depth-1)
	}
	return nodes
}

func hasLegalMove(moves []LegalMove, uci string) bool {
	_, ok := findLegalMove(moves, uci)
	return ok
}

func findLegalMove(moves []LegalMove, uci string) (LegalMove, bool) {
	for _, move := range moves {
		if move.UCI == uci {
			return move, true
		}
	}
	return LegalMove{}, false
}
