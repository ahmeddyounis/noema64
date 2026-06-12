package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/security"
)

type TraceStore struct {
	dir string
	mu  sync.Mutex
}

func NewTraceStore(dir string) *TraceStore {
	if dir == "" {
		dir = "logs"
	}
	return &TraceStore{dir: dir}
}

func (s *TraceStore) AppendDecision(ctx context.Context, trace *decision.MoveDecision) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if trace == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	record := map[string]any{
		"schema_version": "1.0",
		"event_type":     "move_decision",
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"engine_version": "0.1.0",
		"trace":          trace,
	}
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	b = []byte(security.RedactSecrets(string(b)) + "\n")
	path := filepath.Join(s.dir, trace.GameID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}
