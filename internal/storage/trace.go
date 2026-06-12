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
	"github.com/ahmedyounis/noema64/internal/strategy"
)

type TraceStore struct {
	dir      string
	filePath string
	mu       sync.Mutex
}

func NewTraceStore(dir string) *TraceStore {
	if dir == "" {
		dir = "logs"
	}
	return &TraceStore{dir: dir}
}

func NewTraceFileStore(path string) *TraceStore {
	return &TraceStore{filePath: path}
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
	record := decisionTraceRecord(trace)
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	b = []byte(security.RedactSecrets(string(b)) + "\n")
	path := s.filePath
	if path == "" {
		if err := os.MkdirAll(s.dir, 0o700); err != nil {
			return err
		}
		path = filepath.Join(s.dir, trace.GameID+".jsonl")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func decisionTraceRecord(trace *decision.MoveDecision) map[string]any {
	promptVersion := trace.Provider.PromptVersion
	if promptVersion == "" {
		promptVersion = strategy.PromptVersion
	}
	return map[string]any{
		"schema_version":    "1.0",
		"event_type":        "move_decision",
		"timestamp":         time.Now().UTC().Format(time.RFC3339Nano),
		"engine_version":    "0.1.0",
		"game_id":           trace.GameID,
		"ply":               trace.Ply,
		"fen_before":        trace.FENBefore,
		"legal_moves_count": trace.LegalMovesCount,
		"mode":              trace.Mode,
		"provider":          trace.Provider.Name,
		"model":             trace.Provider.Model,
		"prompt_version":    promptVersion,
		"llm_raw_available": trace.Provider.RawAvailable,
		"llm_parse_status":  trace.Provider.ParseStatus,
		"selected_move":     trace.SelectedMove.UCI,
		"fallback_used":     trace.FallbackUsed,
		"candidate_moves":   trace.CandidateMoves,
		"verifier_result":   trace.VerifierTrace,
		"stages":            trace.Stages,
		"strategy_before":   trace.StrategyBefore,
		"strategy_after":    trace.StrategyAfter,
		"timing_ms":         traceTimingRecord(trace.Timing),
		"trace":             trace,
	}
}

func traceTimingRecord(timing decision.Timing) map[string]int64 {
	return map[string]int64{
		"total":    timing.TotalMS,
		"llm":      timing.ProviderMS,
		"verifier": timing.VerifierMS,
		"search":   timing.SearchMS,
		"other":    timing.OtherMS,
	}
}
