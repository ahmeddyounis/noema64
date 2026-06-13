package experiments

import (
	"context"
	"testing"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestTournamentRunsRatingPool(t *testing.T) {
	summary, err := (Runner{MaxPlies: 8}).Tournament(context.Background(), []TournamentEntrant{
		{ID: "pure", Mode: strategy.ModePure},
		{ID: "hybrid", Mode: strategy.ModeHybrid},
	}, 2, 64)
	if err != nil {
		t.Fatalf("tournament: %v", err)
	}
	if summary.SchemaVersion != TournamentSchemaVersion || summary.GamesPlayed != 2 || len(summary.Results) != 2 {
		t.Fatalf("bad summary: %+v", summary)
	}
	if len(summary.Ratings) != 2 {
		t.Fatalf("ratings = %d, want 2: %+v", len(summary.Ratings), summary.Ratings)
	}
	for _, rating := range summary.Ratings {
		if rating.Games != 2 || rating.Elo == 0 {
			t.Fatalf("rating not updated: %+v", rating)
		}
	}
}

func TestTournamentRequiresTwoEntrants(t *testing.T) {
	if _, err := (Runner{}).Tournament(context.Background(), []TournamentEntrant{{ID: "only", Mode: strategy.ModePure}}, 1, 64); err == nil {
		t.Fatal("expected one-entrant tournament to fail")
	}
}
