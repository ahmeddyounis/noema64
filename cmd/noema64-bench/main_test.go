package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/experiments"
)

func TestRequestedGamesDefaults(t *testing.T) {
	if got := requestedGames(0, false); got != 100 {
		t.Fatalf("random default games = %d, want 100", got)
	}
	if got := requestedGames(0, true); got != 20 {
		t.Fatalf("mode default games = %d, want 20", got)
	}
	if got := requestedGames(7, true); got != 7 {
		t.Fatalf("explicit games = %d, want 7", got)
	}
}

func TestWriteArtifacts(t *testing.T) {
	dir := t.TempDir()
	summary := experiments.Summary{
		SchemaVersion:  "1.0",
		GamesRequested: 1,
		GamesCompleted: 1,
		Results:        []experiments.GameSummary{{GameIndex: 1, Plies: 12, Outcome: "draw"}},
	}
	if err := writeArtifacts(dir, summary, false, 1, 64); err != nil {
		t.Fatalf("write artifacts: %v", err)
	}
	for _, name := range []string{"config.yaml", "summary.json", "summary.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	config, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(config), `benchmark: "random"`) || !strings.Contains(string(config), "seed: 64") {
		t.Fatalf("unexpected config:\n%s", string(config))
	}
	csv, err := os.ReadFile(filepath.Join(dir, "summary.csv"))
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if !strings.Contains(string(csv), "benchmark,mode,game_index") || !strings.Contains(string(csv), "random,,1") {
		t.Fatalf("unexpected csv:\n%s", string(csv))
	}
}
