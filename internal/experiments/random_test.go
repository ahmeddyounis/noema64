package experiments

import (
	"context"
	"testing"
	"time"
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
