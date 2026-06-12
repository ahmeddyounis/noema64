package experiments

import (
	"context"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestRandomBenchmarkSample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	summary, err := (Runner{}).RandomLegalBenchmark(ctx, 2, 64)
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

func TestRandomModeBenchmarkCoversCoreModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	summary, err := (Runner{}).RandomLegalModeBenchmark(ctx, 1, 64, nil)
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
