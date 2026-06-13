package chesscore

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	chess "github.com/corentings/chess/v2"
)

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

func TestImportedJSONOpeningBookSuggestions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.json")
	if err := os.WriteFile(path, []byte(`{
		"name":"club",
		"entries":[{"line":"","move":"b1c3","name":"Van Geet","weight":0.4}]
	}`), 0o600); err != nil {
		t.Fatalf("write book: %v", err)
	}
	book, err := ImportOpeningBook(path)
	if err != nil {
		t.Fatalf("import book: %v", err)
	}
	suggestions := OpeningBookSuggestionsWithImports(NewGame(), []ImportedOpeningBook{book})
	if !containsBookMove(suggestions, "b1c3", "club") {
		t.Fatalf("missing imported suggestion: %+v", suggestions)
	}
}

func TestImportedPolyglotOpeningBookSuggestions(t *testing.T) {
	game := NewGame()
	move, err := chess.UCINotation{}.Decode(nil, "e2e4")
	if err != nil {
		t.Fatalf("decode move: %v", err)
	}
	entry := make([]byte, 16)
	binary.BigEndian.PutUint64(entry[0:8], game.PolyglotKey())
	binary.BigEndian.PutUint16(entry[8:10], chess.MoveToPolyglot(*move))
	binary.BigEndian.PutUint16(entry[10:12], 42)
	path := filepath.Join(t.TempDir(), "book.bin")
	if err := os.WriteFile(path, entry, 0o600); err != nil {
		t.Fatalf("write polyglot: %v", err)
	}
	book, err := ImportOpeningBook(path)
	if err != nil {
		t.Fatalf("import polyglot: %v", err)
	}
	suggestions := OpeningBookSuggestionsWithImports(game, []ImportedOpeningBook{book})
	if !containsBookMove(suggestions, "e2e4", "book") {
		t.Fatalf("missing polyglot suggestion: %+v", suggestions)
	}
}

func containsBookMove(entries []OpeningBookEntry, move string, source string) bool {
	for _, entry := range entries {
		if entry.Move == move && entry.Source == source {
			return true
		}
	}
	return false
}
