package experiments

import (
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestRandomBenchmarkSample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	summary, err := (Runner{MaxPlies: 32}).RandomLegalBenchmark(ctx, 2, 64)
	if err != nil {
		t.Fatalf("benchmark: %v", err)
	}
	if summary.GamesCompleted != 2 {
		t.Fatalf("completed = %d, want 2", summary.GamesCompleted)
	}
	if summary.IllegalFinalMoves != 0 {
		t.Fatalf("illegal final moves = %d", summary.IllegalFinalMoves)
	}
}

func TestBenchmarkCSVExportsRows(t *testing.T) {
	summary := Summary{
		SchemaVersion:     "1.0",
		GamesRequested:    2,
		GamesCompleted:    2,
		IllegalFinalMoves: 0,
		TotalPlies:        12,
		Results: []GameSummary{
			{GameIndex: 1, Plies: 5, Outcome: "checkmate", FallbacksUsed: 1},
			{GameIndex: 2, Plies: 7, Outcome: "adjudicated_draw", Adjudicated: true},
		},
	}

	out, err := SummaryCSV(summary)
	if err != nil {
		t.Fatalf("summary csv: %v", err)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("csv rows = %d, want header + 2 results:\n%s", len(rows), out)
	}
	if rows[0][0] != "benchmark" || rows[1][0] != "random" || rows[1][2] != "1" || rows[2][4] != "adjudicated_draw" {
		t.Fatalf("unexpected csv rows: %#v", rows)
	}
}

func TestModeBenchmarkCSVExportsModeRows(t *testing.T) {
	out, err := ModeBenchmarkCSV(ModeBenchmarkSummary{
		SchemaVersion: "1.0",
		GamesPerMode:  1,
		Seed:          64,
		Results: []ModeBenchmarkResult{
			{Mode: strategy.ModePure, Summary: Summary{GamesRequested: 1, GamesCompleted: 1, Results: []GameSummary{{GameIndex: 1, Outcome: "draw"}}}},
			{Mode: strategy.ModeHybrid, Summary: Summary{GamesRequested: 1, GamesCompleted: 1, Results: []GameSummary{{GameIndex: 1, Outcome: "checkmate"}}}},
		},
	})
	if err != nil {
		t.Fatalf("mode csv: %v", err)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 3 || rows[1][1] != "pure" || rows[2][1] != "hybrid" {
		t.Fatalf("unexpected mode csv rows: %#v", rows)
	}
}

func TestRandomModeBenchmarkCoversCoreModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	summary, err := (Runner{MaxPlies: 32}).RandomLegalModeBenchmark(ctx, 1, 64, nil)
	if err != nil {
		t.Fatalf("mode benchmark: %v", err)
	}
	if summary.GamesPerMode != 1 {
		t.Fatalf("games per mode = %d, want 1", summary.GamesPerMode)
	}
	if summary.Seed != 64 {
		t.Fatalf("seed = %d, want 64", summary.Seed)
	}
	wantModes := []strategy.EngineMode{strategy.ModePure, strategy.ModeBlunderguard, strategy.ModeHybrid}
	if len(summary.Results) != len(wantModes) {
		t.Fatalf("results = %d, want %d", len(summary.Results), len(wantModes))
	}
	for i, want := range wantModes {
		result := summary.Results[i]
		if result.Mode != want {
			t.Fatalf("result %d mode = %s, want %s", i, result.Mode, want)
		}
		if result.Summary.GamesCompleted != 1 {
			t.Fatalf("%s completed = %d, want 1", result.Mode, result.Summary.GamesCompleted)
		}
		if result.Summary.IllegalFinalMoves != 0 {
			t.Fatalf("%s illegal final moves = %d", result.Mode, result.Summary.IllegalFinalMoves)
		}
	}
}

func TestPositionSuiteRunsDeterministicPositions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	summary, err := (Runner{}).PositionSuite(ctx, []SuitePosition{
		{Name: "start", FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{Name: "endgame", FEN: "8/5pk1/6p1/4P3/5P2/6K1/8/8 w - - 0 42"},
	})
	if err != nil {
		t.Fatalf("position suite: %v", err)
	}
	if summary.SchemaVersion != PositionSuiteSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", summary.SchemaVersion, PositionSuiteSchemaVersion)
	}
	if summary.PositionsRequested != 2 || summary.PositionsAnalyzed != 2 || summary.EngineErrors != 0 {
		t.Fatalf("unexpected position suite summary: %+v", summary)
	}
	for _, result := range summary.Results {
		if result.SelectedMove == "" || result.CandidateCount == 0 || result.Provider == "" {
			t.Fatalf("incomplete position result: %+v", result)
		}
	}
}

func TestPositionSuiteRecordsPositionErrors(t *testing.T) {
	summary, err := (Runner{}).PositionSuite(context.Background(), []SuitePosition{{Name: "bad", FEN: "not a fen"}})
	if err != nil {
		t.Fatalf("position suite should keep per-position errors: %v", err)
	}
	if summary.PositionsAnalyzed != 0 || summary.EngineErrors != 1 {
		t.Fatalf("unexpected error summary: %+v", summary)
	}
	if len(summary.Results) != 1 || summary.Results[0].EngineError == "" {
		t.Fatalf("missing per-position error: %+v", summary.Results)
	}
}
