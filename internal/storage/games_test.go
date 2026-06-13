package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/engine"
)

func TestGameStoreWritesRedactedSnapshotAndListsRecentGames(t *testing.T) {
	dir := t.TempDir()
	e := engine.New(engine.Options{})
	state, err := e.NewGame(context.Background(), engine.NewGameOptions{Side: "white"})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	state.LastDecision = &decision.MoveDecision{
		GameID: state.Snapshot.GameID,
		Provider: decision.ProviderTrace{
			Error: "api_key: secret-value",
		},
	}

	store := NewGameStore(dir)
	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("save: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("snapshot files = %d, want 1", len(files))
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if strings.Contains(string(b), "secret-value") {
		t.Fatalf("snapshot leaked secret: %s", string(b))
	}
	if !strings.Contains(string(b), `"schema_version": "game-state.v1"`) {
		t.Fatalf("snapshot missing game state schema version: %s", string(b))
	}

	records, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || records[0].GameID != state.Snapshot.GameID {
		t.Fatalf("unexpected records: %+v", records)
	}
	latest, err := store.LoadLatest(context.Background())
	if err != nil {
		t.Fatalf("load latest: %v", err)
	}
	if latest.GameID != state.Snapshot.GameID {
		t.Fatalf("latest game = %s, want %s", latest.GameID, state.Snapshot.GameID)
	}
	loaded, err := store.Load(context.Background(), state.Snapshot.GameID)
	if err != nil {
		t.Fatalf("load game: %v", err)
	}
	if loaded.GameID != state.Snapshot.GameID {
		t.Fatalf("loaded game = %s, want %s", loaded.GameID, state.Snapshot.GameID)
	}
}

func TestGameStoreListSkipsCorruptRecords(t *testing.T) {
	dir := t.TempDir()
	e := engine.New(engine.Options{})
	state, err := e.NewGame(context.Background(), engine.NewGameOptions{Side: "white"})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	store := NewGameStore(dir)
	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("save: %v", err)
	}
	time.Sleep(time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt record: %v", err)
	}

	records, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || records[0].GameID != state.Snapshot.GameID {
		t.Fatalf("records = %+v, want only valid saved game", records)
	}
	latest, err := store.LoadLatest(context.Background())
	if err != nil {
		t.Fatalf("load latest: %v", err)
	}
	if latest.GameID != state.Snapshot.GameID {
		t.Fatalf("latest game = %s, want %s", latest.GameID, state.Snapshot.GameID)
	}
}

func TestGameStoreRejectsUnknownFutureSchemaOnExplicitLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "future.json"), []byte(`{"schema_version":"game-record.v99","game_id":"future"}`), 0o600); err != nil {
		t.Fatalf("write future record: %v", err)
	}
	store := NewGameStore(dir)
	if _, err := store.Load(context.Background(), "future"); err == nil {
		t.Fatal("expected unknown future game record schema to fail")
	}
}
