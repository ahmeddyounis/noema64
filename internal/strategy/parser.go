package strategy

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ParseResult struct {
	Decision DecisionOutput `json:"decision"`
	Status   string         `json:"status"`
	RawKept  bool           `json:"raw_kept"`
	Error    string         `json:"error,omitempty"`
}

func ParseDecision(raw string) ParseResult {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParseResult{Status: "empty", Error: "empty provider response"}
	}
	var out DecisionOutput
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		if err := validateDecision(out); err != nil {
			return ParseResult{Status: "schema_invalid", Error: err.Error()}
		}
		return ParseResult{Decision: out, Status: "ok"}
	}
	extracted, err := firstJSONObject(raw)
	if err != nil {
		return ParseResult{Status: "json_invalid", Error: err.Error()}
	}
	if err := json.Unmarshal([]byte(extracted), &out); err != nil {
		return ParseResult{Status: "json_invalid", Error: err.Error()}
	}
	if err := validateDecision(out); err != nil {
		return ParseResult{Status: "schema_invalid", Error: err.Error()}
	}
	return ParseResult{Decision: out, Status: "extracted_json"}
}

func validateDecision(out DecisionOutput) error {
	if out.SchemaVersion == "" {
		return errors.New("missing schema_version")
	}
	if len(out.CandidateMoves) == 0 {
		return errors.New("candidate_moves must not be empty")
	}
	for i, candidate := range out.CandidateMoves {
		if strings.TrimSpace(candidate.UCI) == "" && strings.TrimSpace(candidate.SAN) == "" {
			return fmt.Errorf("candidate %d has no move notation", i)
		}
	}
	return nil
}

func firstJSONObject(s string) (string, error) {
	start := strings.Index(s, "{")
	if start < 0 {
		return "", errors.New("no JSON object start")
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}
	return "", errors.New("unterminated JSON object")
}
