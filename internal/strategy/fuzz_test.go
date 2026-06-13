package strategy

import (
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/chesscore"
)

func FuzzParseDecisionDoesNotPanic(f *testing.F) {
	for _, seed := range []string{
		"",
		"not json",
		`{"schema_version":"decision-output.v1.2"}`,
		`prefix {"schema_version":"decision-output.v1.2","previous_plan_status":"continue","position_summary":"ok","strategy_update":{"plan_summary":"develop","phase":"opening","confidence":0.6},"candidate_moves":[{"uci":"e2e4","purpose":"center","confidence":0.7}],"do_not_play":[]} suffix`,
		`{"schema_version":"decision-output.v9","previous_plan_status":"continue","position_summary":"ok","strategy_update":{"plan_summary":"develop"},"candidate_moves":[{"uci":"e2e4"}]}`,
		strings.Repeat("{", 32),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		parsed := ParseDecision(raw)
		switch parsed.Status {
		case "ok", "extracted_json":
			if parsed.Decision.SchemaVersion != DecisionSchemaVersion {
				t.Fatalf("accepted unsupported schema %q from %q", parsed.Decision.SchemaVersion, raw)
			}
			if len(parsed.Decision.CandidateMoves) == 0 {
				t.Fatalf("accepted decision without candidates from %q", raw)
			}
		case "empty", "json_invalid", "schema_invalid":
		default:
			t.Fatalf("unexpected parse status %q from %q", parsed.Status, raw)
		}
	})
}

func FuzzNormalizeCandidateNotationDoesNotPanic(f *testing.F) {
	for _, seed := range []string{
		"e2e4",
		"Nf3!",
		"Ng1-f3",
		"O-O",
		"0-0-0",
		"e2e5",
		"🙂",
		strings.Repeat("Q", 128),
	} {
		f.Add(seed)
	}
	game := chesscore.NewGame()

	f.Fuzz(func(t *testing.T, raw string) {
		candidates, attempts := NormalizeCandidates(game, []CandidateMove{
			{UCI: raw, Purpose: "fuzz uci", LLMConfidence: 2},
			{SAN: raw, Purpose: "fuzz san", LLMConfidence: -1},
		})
		for _, candidate := range candidates {
			if !game.IsLegalUCI(candidate.UCI) {
				t.Fatalf("normalized illegal move %q from raw %q attempts=%+v", candidate.UCI, raw, attempts)
			}
			if candidate.LLMConfidence < 0 || candidate.LLMConfidence > 1 {
				t.Fatalf("confidence not clamped for %q: %+v", raw, candidate)
			}
		}
	})
}
