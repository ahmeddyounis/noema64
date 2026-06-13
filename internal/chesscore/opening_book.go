package chesscore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chess "github.com/corentings/chess/v2"
)

const OpeningBookSchemaVersion = "opening-book.v1"

type OpeningBookEntry struct {
	SchemaVersion string  `json:"schema_version"`
	Line          string  `json:"line"`
	Move          string  `json:"move"`
	Name          string  `json:"name"`
	Weight        float64 `json:"weight"`
	Source        string  `json:"source"`
}

type ImportedOpeningBook struct {
	SchemaVersion string                        `json:"schema_version"`
	Name          string                        `json:"name"`
	Path          string                        `json:"path"`
	Format        string                        `json:"format"`
	EntryCount    int                           `json:"entry_count"`
	Matchable     bool                          `json:"matchable"`
	LineEntries   map[string][]OpeningBookEntry `json:"line_entries,omitempty"`
	PolyglotMoves map[uint64][]OpeningBookEntry `json:"polyglot_moves,omitempty"`
}

type openingBookJSON struct {
	SchemaVersion string                        `json:"schema_version"`
	Name          string                        `json:"name"`
	Entries       []OpeningBookEntry            `json:"entries"`
	Lines         map[string][]OpeningBookEntry `json:"lines"`
}

func OpeningBookSuggestions(game *Game) []OpeningBookEntry {
	if game == nil || game.Outcome().Status != "ongoing" {
		return nil
	}
	key := strings.Join(game.AppliedUCI(), " ")
	entries := defaultOpeningBook()[key]
	if len(entries) == 0 {
		return nil
	}
	out := []OpeningBookEntry{}
	for _, entry := range entries {
		if game.IsLegalUCI(entry.Move) {
			entry.SchemaVersion = OpeningBookSchemaVersion
			entry.Line = key
			entry.Source = "noema64_builtin"
			out = append(out, entry)
		}
	}
	return out
}

func OpeningBookSuggestionsWithImports(game *Game, books []ImportedOpeningBook) []OpeningBookEntry {
	out := OpeningBookSuggestions(game)
	if game == nil || game.Outcome().Status != "ongoing" {
		return out
	}
	line := strings.Join(game.AppliedUCI(), " ")
	for _, book := range books {
		for _, entry := range book.LineEntries[line] {
			if game.IsLegalUCI(entry.Move) {
				entry.SchemaVersion = OpeningBookSchemaVersion
				entry.Line = line
				if entry.Source == "" {
					entry.Source = book.Name
				}
				out = append(out, entry)
			}
		}
		if len(book.PolyglotMoves) > 0 {
			for _, entry := range book.PolyglotMoves[game.PolyglotKey()] {
				if game.IsLegalUCI(entry.Move) {
					entry.SchemaVersion = OpeningBookSchemaVersion
					entry.Line = line
					if entry.Source == "" {
						entry.Source = book.Name
					}
					out = append(out, entry)
				}
			}
		}
	}
	return out
}

func ImportOpeningBook(path string) (ImportedOpeningBook, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ImportedOpeningBook{}, fmt.Errorf("opening book path is required")
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return importOpeningBookJSON(path)
	case ".bin", ".polyglot":
		return importOpeningBookPolyglot(path)
	default:
		return ImportedOpeningBook{}, fmt.Errorf("unsupported opening book format %q", ext)
	}
}

func importOpeningBookJSON(path string) (ImportedOpeningBook, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ImportedOpeningBook{}, err
	}
	var wrapped openingBookJSON
	if err := json.Unmarshal(b, &wrapped); err != nil {
		var entries []OpeningBookEntry
		if altErr := json.Unmarshal(b, &entries); altErr != nil {
			return ImportedOpeningBook{}, err
		}
		wrapped.Entries = entries
	}
	name := strings.TrimSpace(wrapped.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	book := ImportedOpeningBook{
		SchemaVersion: OpeningBookSchemaVersion,
		Name:          name,
		Path:          path,
		Format:        "json",
		Matchable:     true,
		LineEntries:   map[string][]OpeningBookEntry{},
	}
	for line, entries := range wrapped.Lines {
		for _, entry := range entries {
			book.addLineEntry(line, entry)
		}
	}
	for _, entry := range wrapped.Entries {
		book.addLineEntry(entry.Line, entry)
	}
	return book, nil
}

func importOpeningBookPolyglot(path string) (ImportedOpeningBook, error) {
	file, err := os.Open(path)
	if err != nil {
		return ImportedOpeningBook{}, err
	}
	defer file.Close()
	poly, err := chess.LoadFromReader(file)
	if err != nil {
		return ImportedOpeningBook{}, err
	}
	book := ImportedOpeningBook{
		SchemaVersion: OpeningBookSchemaVersion,
		Name:          strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Path:          path,
		Format:        "polyglot",
		Matchable:     true,
		PolyglotMoves: map[uint64][]OpeningBookEntry{},
	}
	for key, moves := range poly.ToMoveMap() {
		for _, move := range moves {
			book.PolyglotMoves[key] = append(book.PolyglotMoves[key], OpeningBookEntry{
				SchemaVersion: OpeningBookSchemaVersion,
				Move:          move.Move.String(),
				Name:          "Polyglot move",
				Weight:        float64(move.Weight),
				Source:        book.Name,
			})
			book.EntryCount++
		}
	}
	return book, nil
}

func (book *ImportedOpeningBook) addLineEntry(line string, entry OpeningBookEntry) {
	line = strings.TrimSpace(line)
	entry.Line = line
	entry.SchemaVersion = OpeningBookSchemaVersion
	if entry.Source == "" {
		entry.Source = book.Name
	}
	book.LineEntries[line] = append(book.LineEntries[line], entry)
	book.EntryCount++
}

func (g *Game) PolyglotKey() uint64 {
	if g == nil {
		return 0
	}
	hash, err := chess.NewZobristHasher().HashPosition(g.FEN())
	if err != nil {
		return 0
	}
	return chess.ZobristHashToUint64(hash)
}

func defaultOpeningBook() map[string][]OpeningBookEntry {
	return map[string][]OpeningBookEntry{
		"": {
			{Move: "e2e4", Name: "Open Game", Weight: 1.00},
			{Move: "d2d4", Name: "Queen's Pawn", Weight: 0.92},
			{Move: "g1f3", Name: "Reti setup", Weight: 0.72},
			{Move: "c2c4", Name: "English", Weight: 0.68},
		},
		"e2e4": {
			{Move: "c7c5", Name: "Sicilian Defense", Weight: 0.95},
			{Move: "e7e5", Name: "Open Game", Weight: 0.90},
			{Move: "e7e6", Name: "French Defense", Weight: 0.74},
			{Move: "c7c6", Name: "Caro-Kann", Weight: 0.70},
		},
		"d2d4": {
			{Move: "g8f6", Name: "Indian defenses", Weight: 0.92},
			{Move: "d7d5", Name: "Closed Game", Weight: 0.88},
		},
		"e2e4 c7c5": {
			{Move: "g1f3", Name: "Open Sicilian", Weight: 0.90},
			{Move: "b1c3", Name: "Closed Sicilian", Weight: 0.62},
		},
		"e2e4 e7e5": {
			{Move: "g1f3", Name: "King's Knight Opening", Weight: 0.96},
			{Move: "f1c4", Name: "Bishop's Opening", Weight: 0.64},
		},
	}
}
