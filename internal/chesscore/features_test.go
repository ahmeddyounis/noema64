package chesscore

import (
	"strings"
	"testing"
)

func TestFeaturesDetectCheckAndKingSafety(t *testing.T) {
	game, err := FromFEN("4k3/8/8/8/8/8/4r3/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("fen: %v", err)
	}
	features := game.Features()
	if !features.InCheck {
		t.Fatalf("in_check = false, want true: %+v", features)
	}
	if !hasPrefix(features.KingSafety, "king_attacked_by:e2:black_rook") {
		t.Fatalf("king safety missing rook attacker: %+v", features.KingSafety)
	}
	if len(features.Captures) == 0 || len(features.Threats) == 0 {
		t.Fatalf("expected legal capture threat while resolving check: %+v", features)
	}
}

func TestFeaturesDetectPinnedAndHangingPieces(t *testing.T) {
	pinnedGame, err := FromFEN("4r1k1/8/8/8/8/8/4N3/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("pinned fen: %v", err)
	}
	pinned := pinnedGame.Features()
	if !hasPrefix(pinned.PinnedPieces, "e2:white_knight pinned to king") {
		t.Fatalf("pinned pieces = %+v, want knight on e2", pinned.PinnedPieces)
	}
	if pinned.InCheck {
		t.Fatalf("blocked rook should not put king in check: %+v", pinned)
	}

	hangingGame, err := FromFEN("6k1/8/8/5n2/3Q4/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("hanging fen: %v", err)
	}
	hanging := hangingGame.Features()
	if !hasPrefix(hanging.HangingPieces, "d4:white_queen attacked by f5:black_knight") {
		t.Fatalf("hanging pieces = %+v, want undefended queen on d4", hanging.HangingPieces)
	}
}

func hasPrefix(items []string, prefix string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
