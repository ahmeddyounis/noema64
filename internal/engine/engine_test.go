package engine

import (
	"context"
	"testing"

	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestEngineFallbackOnInvalidProviderJSON(t *testing.T) {
	e := New(Options{
		Mode:     strategy.ModePure,
		Provider: providers.MockProvider{Behavior: "invalid_json"},
	})
	dec, state, err := e.ChooseMove(context.Background())
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if !dec.FallbackUsed {
		t.Fatal("expected fallback decision")
	}
	if state.Snapshot.Ply != 1 {
		t.Fatalf("ply = %d, want 1", state.Snapshot.Ply)
	}
}

func TestEngineRejectsIllegalUserMove(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e5"); err == nil {
		t.Fatal("expected illegal move error")
	}
}
