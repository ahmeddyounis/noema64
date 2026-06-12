package appsvc

import (
	"context"
	"testing"
)

func TestImportFENAndPGN(t *testing.T) {
	app := NewApplication("")
	fenState, appErr := app.ImportFEN(context.Background(), "8/P7/8/8/8/8/8/4k2K w - - 0 1")
	if appErr != nil {
		t.Fatalf("import fen: %v", appErr)
	}
	if fenState.Snapshot.FEN == "" || fenState.Snapshot.SideToMove != "white" {
		t.Fatalf("unexpected fen state: %+v", fenState.Snapshot)
	}

	pgnState, appErr := app.ImportPGN(context.Background(), "1. e4 e5 2. Nf3 Nc6 *")
	if appErr != nil {
		t.Fatalf("import pgn: %v", appErr)
	}
	if len(pgnState.Snapshot.MoveHistory) != 4 {
		t.Fatalf("history length = %d, want 4", len(pgnState.Snapshot.MoveHistory))
	}
}
