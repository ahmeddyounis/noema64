package chesscore

import "strings"

const OpeningBookSchemaVersion = "opening-book.v1"

type OpeningBookEntry struct {
	SchemaVersion string  `json:"schema_version"`
	Line          string  `json:"line"`
	Move          string  `json:"move"`
	Name          string  `json:"name"`
	Weight        float64 `json:"weight"`
	Source        string  `json:"source"`
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
