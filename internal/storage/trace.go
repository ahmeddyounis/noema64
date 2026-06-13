package storage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		path = filepath.Join(s.dir, traceFileName(trace.GameID))
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

func (s *TraceStore) ReadGame(ctx context.Context, gameID string) (string, error) {
	text, err := s.readGame(ctx, gameID)
	if err != nil {
		return "", err
	}
	return stripRawProviderData(text), nil
}

func (s *TraceStore) ReadGameDebug(ctx context.Context, gameID string) (string, error) {
	return s.readGame(ctx, gameID)
}

func (s *TraceStore) readGame(ctx context.Context, gameID string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	path := s.filePath
	if path == "" {
		gameID = strings.TrimSpace(gameID)
		if gameID == "" {
			return "", fmt.Errorf("game id is required")
		}
		path = filepath.Join(s.dir, traceFileName(gameID))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return security.RedactSecrets(string(b)), nil
}

func stripRawProviderData(jsonl string) string {
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(jsonl))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			out.WriteByte('\n')
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		stripRawFields(value)
		b, err := json.Marshal(value)
		if err != nil {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		out.Write(b)
		out.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return jsonl
	}
	return out.String()
}

func stripRawFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, "raw_prompt")
		delete(typed, "raw_response")
		for _, child := range typed {
			stripRawFields(child)
		}
	case []any:
		for _, child := range typed {
			stripRawFields(child)
		}
	}
}

func decisionTraceRecord(trace *decision.MoveDecision) map[string]any {
	promptID := trace.Provider.PromptID
	if promptID == "" {
		promptID = strategy.PromptID
	}
	promptVersion := trace.Provider.PromptVersion
	if promptVersion == "" {
		promptVersion = strategy.PromptVersion
	}
	promptSchemaVersion := trace.Provider.PromptSchemaVersion
	if promptSchemaVersion == "" {
		promptSchemaVersion = strategy.PromptTemplateSchemaVersion
	}
	decisionSchemaVersion := trace.Provider.DecisionSchemaVersion
	if decisionSchemaVersion == "" {
		decisionSchemaVersion = strategy.DecisionSchemaVersion
	}
	return map[string]any{
		"schema_version":          "1.0",
		"event_type":              "move_decision",
		"timestamp":               time.Now().UTC().Format(time.RFC3339Nano),
		"engine_version":          "0.1.0",
		"game_id":                 trace.GameID,
		"ply":                     trace.Ply,
		"fen_before":              trace.FENBefore,
		"legal_moves_count":       trace.LegalMovesCount,
		"mode":                    trace.Mode,
		"provider":                trace.Provider.Name,
		"model":                   trace.Provider.Model,
		"prompt_id":               promptID,
		"prompt_version":          promptVersion,
		"prompt_schema_version":   promptSchemaVersion,
		"decision_schema_version": decisionSchemaVersion,
		"temperature":             trace.Provider.Temperature,
		"max_tokens":              trace.Provider.MaxTokens,
		"retry_count":             trace.Provider.RetryCount,
		"llm_raw_available":       trace.Provider.RawAvailable,
		"llm_parse_status":        trace.Provider.ParseStatus,
		"selected_move":           trace.SelectedMove.UCI,
		"analysis_only":           trace.AnalysisOnly,
		"fallback_used":           trace.FallbackUsed,
		"candidate_moves":         trace.CandidateMoves,
		"verifier_result":         trace.VerifierTrace,
		"stages":                  trace.Stages,
		"strategy_before_hash":    strategyMemoryHash(trace.StrategyBefore),
		"strategy_after_hash":     strategyMemoryHash(trace.StrategyAfter),
		"strategy_before":         trace.StrategyBefore,
		"strategy_after":          trace.StrategyAfter,
		"timing_ms":               traceTimingRecord(trace.Timing),
		"trace":                   trace,
	}
}

func strategyMemoryHash(memory strategy.StrategyMemory) string {
	b, err := json.Marshal(memory)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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

func traceFileName(gameID string) string {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return "current.jsonl"
	}
	var b strings.Builder
	for _, r := range gameID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "current.jsonl"
	}
	return b.String() + ".jsonl"
}
