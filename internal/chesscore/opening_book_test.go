package chesscore

import "testing"

func TestOpeningBookSuggestionsFollowLegalLine(t *testing.T) {
	game := NewGame()
	start := OpeningBookSuggestions(game)
	if len(start) == 0 || start[0].SchemaVersion != OpeningBookSchemaVersion {
		t.Fatalf("missing start book suggestions: %+v", start)
	}
	if _, err := game.ApplyUCI("e2e4"); err != nil {
		t.Fatalf("apply e4: %v", err)
	}
	black := OpeningBookSuggestions(game)
	if len(black) == 0 || black[0].Move == "e2e4" {
		t.Fatalf("missing black book suggestions: %+v", black)
	}
}
