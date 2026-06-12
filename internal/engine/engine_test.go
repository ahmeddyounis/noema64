package engine

import (
	"context"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestEngineFallbackOnProviderFaults(t *testing.T) {
	tests := []struct {
		name         string
		behavior     string
		timeout      time.Duration
		wantFallback bool
	}{
		{name: "invalid json", behavior: "invalid_json", wantFallback: true},
		{name: "empty response", behavior: "empty", wantFallback: true},
		{name: "provider error", behavior: "error", wantFallback: true},
		{name: "illegal only", behavior: "illegal", wantFallback: true},
		{name: "slow timeout", behavior: "slow", timeout: time.Millisecond, wantFallback: true},
		{name: "mixed illegal and legal", behavior: "mixed_illegal", wantFallback: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Mode:     strategy.ModePure,
				Provider: providers.MockProvider{Behavior: tt.behavior},
			}
			if tt.timeout > 0 {
				opts.MoveTimeout = tt.timeout
			}
			e := New(opts)
			dec, state, err := e.ChooseMove(context.Background())
			if err != nil {
				t.Fatalf("choose: %v", err)
			}
			if dec.FallbackUsed != tt.wantFallback {
				t.Fatalf("fallback = %t, want %t", dec.FallbackUsed, tt.wantFallback)
			}
			if state.Snapshot.Ply != 1 {
				t.Fatalf("ply = %d, want 1", state.Snapshot.Ply)
			}
			if state.Snapshot.MoveHistory[0].UCI == "" {
				t.Fatalf("selected invalid UCI: %+v", state.Snapshot.MoveHistory[0])
			}
		})
	}
}

func TestEngineRejectsIllegalUserMove(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e5"); err == nil {
		t.Fatal("expected illegal move error")
	}
}

func TestEngineUndoClearsFutureHistory(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e4"); err != nil {
		t.Fatalf("move 1: %v", err)
	}
	if _, err := e.ApplyUserMove(context.Background(), "e7e5"); err != nil {
		t.Fatalf("move 2: %v", err)
	}
	state, err := e.Undo(context.Background(), 1)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(state.Snapshot.MoveHistory) != 1 {
		t.Fatalf("history length = %d, want 1", len(state.Snapshot.MoveHistory))
	}
}

func TestEngineClockStateAndTimeoutOutcome(t *testing.T) {
	e := New(Options{})
	state, err := e.NewGame(context.Background(), NewGameOptions{
		Side:        "white",
		TimeControl: TimeControl{InitialMS: 300000, IncrementMS: 2000},
	})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	if !state.Clock.Enabled || state.Clock.WhiteMS != 300000 || state.Clock.BlackMS != 300000 || state.Clock.IncrementMS != 2000 {
		t.Fatalf("unexpected clock: %+v", state.Clock)
	}
	state, err = e.ApplyUserMove(context.Background(), "e2e4")
	if err != nil {
		t.Fatalf("user move: %v", err)
	}
	if state.Clock.WhiteMS != 302000 {
		t.Fatalf("white clock after increment = %d, want 302000", state.Clock.WhiteMS)
	}

	e.clock.WhiteMS = 0
	state, err = e.State(context.Background())
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Snapshot.Outcome.Status != "timeout" || state.Snapshot.Outcome.Winner != "black" {
		t.Fatalf("timeout outcome not represented: %+v", state.Snapshot.Outcome)
	}
	if len(state.Snapshot.LegalMoves) != 0 {
		t.Fatalf("timeout state kept legal moves: %d", len(state.Snapshot.LegalMoves))
	}
	if _, err := e.ApplyUserMove(context.Background(), "e7e5"); err == nil {
		t.Fatal("expected move after timeout to fail")
	}
}

func TestEngineLoadStateRestoresMovesAndStrategyMemory(t *testing.T) {
	e := New(Options{})
	if _, err := e.ApplyUserMove(context.Background(), "e2e4"); err != nil {
		t.Fatalf("user move: %v", err)
	}
	if _, _, err := e.ChooseMove(context.Background()); err != nil {
		t.Fatalf("engine move: %v", err)
	}
	saved, err := e.State(context.Background())
	if err != nil {
		t.Fatalf("state: %v", err)
	}

	restored := New(Options{})
	state, err := restored.LoadState(context.Background(), *saved)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Snapshot.GameID != saved.Snapshot.GameID {
		t.Fatalf("game id = %s, want %s", state.Snapshot.GameID, saved.Snapshot.GameID)
	}
	if len(state.Snapshot.MoveHistory) != len(saved.Snapshot.MoveHistory) {
		t.Fatalf("history length = %d, want %d", len(state.Snapshot.MoveHistory), len(saved.Snapshot.MoveHistory))
	}
	if state.StrategyMemory.LastUpdate.MovePlayed == "" {
		t.Fatalf("strategy memory did not restore last update: %+v", state.StrategyMemory)
	}
	if len(state.Snapshot.LegalMoves) == 0 && state.Snapshot.Outcome.Status == "ongoing" {
		t.Fatalf("ongoing restored game has no legal moves")
	}
}
