package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/security"
)

const gameRecordSchemaVersion = "game-record.v1"

type GameRecord struct {
	SchemaVersion string           `json:"schema_version"`
	SavedAt       string           `json:"saved_at"`
	GameID        string           `json:"game_id"`
	State         engine.GameState `json:"state"`
}

type GameStore struct {
	dir string
	mu  sync.Mutex
}

func NewGameStore(dir string) *GameStore {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("logs", "games")
	}
	return &GameStore{dir: dir}
}

func (s *GameStore) Save(ctx context.Context, state *engine.GameState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if state == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	record := GameRecord{
		SchemaVersion: gameRecordSchemaVersion,
		SavedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		GameID:        state.Snapshot.GameID,
		State:         *state,
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	b = []byte(security.RedactSecrets(string(b)) + "\n")
	path := filepath.Join(s.dir, gameRecordFileName(record.GameID))
	tmp, err := os.CreateTemp(s.dir, "."+gameRecordFileName(record.GameID)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *GameStore) LoadLatest(ctx context.Context) (*GameRecord, error) {
	records, err := s.List(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, os.ErrNotExist
	}
	return &records[0], nil
}

func (s *GameStore) Load(ctx context.Context, gameID string) (*GameRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if strings.TrimSpace(gameID) == "" {
		return nil, os.ErrNotExist
	}
	record, err := readGameRecord(filepath.Join(s.dir, gameRecordFileName(gameID)))
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *GameStore) List(ctx context.Context, limit int) ([]GameRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if limit <= 0 {
		limit = 10
	}
	files, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []GameRecord{}, nil
	}
	type candidate struct {
		path    string
		modTime time.Time
	}
	candidates := make([]candidate, 0, len(files))
	for _, path := range files {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate{path: path, modTime: info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	if limit > len(candidates) {
		limit = len(candidates)
	}
	records := make([]GameRecord, 0, limit)
	for _, item := range candidates {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		record, err := readGameRecord(item.path)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
		if len(records) >= limit {
			break
		}
	}
	return records, nil
}

func readGameRecord(path string) (GameRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return GameRecord{}, err
	}
	var record GameRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return GameRecord{}, err
	}
	if record.SchemaVersion == "" {
		record.SchemaVersion = gameRecordSchemaVersion
	}
	return record, nil
}

func gameRecordFileName(gameID string) string {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		gameID = "current"
	}
	var b strings.Builder
	for _, r := range gameID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "current.json"
	}
	return b.String() + ".json"
}
