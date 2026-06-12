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
	for _, mv := range game.LegalMoves() {
		if mv.UCI == "a7a8q" {
			foundPromotion = true
		}
	}
	if !foundPromotion {
		t.Fatal("expected queen promotion legal move")
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
