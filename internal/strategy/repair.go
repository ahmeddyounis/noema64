package strategy

import (
	"strings"

	"github.com/ahmedyounis/noema64/internal/chesscore"
)

func NormalizeCandidates(game interface {
	NormalizeMove(string) (chesscore.LegalMove, bool)
}, candidates []CandidateMove) ([]CandidateMove, []RepairAttempt) {
	seen := map[string]struct{}{}
	normalized := make([]CandidateMove, 0, len(candidates))
	attempts := make([]RepairAttempt, 0, len(candidates))
	for _, candidate := range candidates {
		raws := []string{candidate.UCI, candidate.SAN}
		var chosen chesscore.LegalMove
		ok := false
		method := "failed"
		for _, raw := range raws {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			mv, legal := game.NormalizeMove(raw)
			attempts = append(attempts, RepairAttempt{
				Raw:        raw,
				Normalized: mv.UCI,
				Method:     repairMethod(raw, mv),
				Legal:      legal,
				Confidence: boolConfidence(legal),
			})
			if legal {
				chosen = mv
				ok = true
				method = repairMethod(raw, mv)
				break
			}
		}
		if !ok {
			continue
		}
		if _, exists := seen[chosen.UCI]; exists {
			continue
		}
		seen[chosen.UCI] = struct{}{}
		candidate.UCI = chosen.UCI
		candidate.SAN = chosen.SAN
		candidate.LegalMove = chosen
		candidate.RepairMethod = method
		candidate.LLMConfidence = clamp(candidate.LLMConfidence, 0, 1)
		if candidate.Purpose == "" {
			candidate.Purpose = "Legal candidate selected by the strategy provider."
		}
		normalized = append(normalized, candidate)
	}
	return normalized, attempts
}

func repairMethod(raw string, mv chesscore.LegalMove) string {
	switch {
	case raw == mv.UCI:
		return "exact_uci"
	case strings.EqualFold(raw, mv.SAN):
		return "san_parse"
	case strings.EqualFold(raw, mv.LAN):
		return "lan_parse"
	default:
		return "notation_repair"
	}
}

func boolConfidence(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}
