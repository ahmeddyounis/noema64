package verifier

import (
	"context"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestStaticVerifierRejectsDirectQueenBlunder(t *testing.T) {
	game, err := chesscore.FromFEN("6k1/8/5n2/8/8/8/8/3QK3 w - - 0 1")
	if err != nil {
		t.Fatalf("fen: %v", err)
	}
	result, err := (StaticVerifier{Enabled: true}).VerifyCandidates(context.Background(), Request{
		Game: game,
		Candidates: []strategy.CandidateMove{
			{UCI: "d1h5"},
		},
		Mode: strategy.ModeBlunderguard,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(result.Candidates))
	}
	got := result.Candidates[0]
	if got.Status != "rejected" || !strings.Contains(got.Reason, "white queen") || !strings.Contains(got.Reason, "f6h5") {
		t.Fatalf("queen blunder result = %+v, want rejected direct capture", got)
	}
	if got.Score.Status != "rejected" {
		t.Fatalf("verifier score = %+v, want rejected", got.Score)
	}
}

func TestStaticVerifierWarnsDirectRookLoss(t *testing.T) {
	game, err := chesscore.FromFEN("6k1/8/5n2/8/8/8/8/4K2R w - - 0 1")
	if err != nil {
		t.Fatalf("fen: %v", err)
	}
	result, err := (StaticVerifier{Enabled: true}).VerifyCandidates(context.Background(), Request{
		Game: game,
		Candidates: []strategy.CandidateMove{
			{UCI: "h1h5"},
		},
		Mode: strategy.ModeBlunderguard,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	got := result.Candidates[0]
	if got.Status != "warning" || !strings.Contains(got.Reason, "white rook") || !strings.Contains(got.Reason, "f6h5") {
		t.Fatalf("rook loss result = %+v, want warning direct capture", got)
	}
}
