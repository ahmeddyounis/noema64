package verifier

import (
	"context"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

type Request struct {
	Game       *chesscore.Game          `json:"-"`
	FEN        string                   `json:"position_fen"`
	Candidates []strategy.CandidateMove `json:"candidates"`
	Mode       strategy.EngineMode      `json:"mode"`
}

type CandidateResult struct {
	UCI      string                 `json:"uci"`
	Status   string                 `json:"status"`
	Reason   string                 `json:"reason"`
	MateRisk bool                   `json:"mate_risk"`
	Details  map[string]string      `json:"details,omitempty"`
	Score    strategy.VerifierScore `json:"score"`
}

type Result struct {
	Enabled    bool              `json:"enabled"`
	Used       bool              `json:"used"`
	Name       string            `json:"name"`
	Candidates []CandidateResult `json:"candidates"`
	Error      string            `json:"error,omitempty"`
}

type Verifier interface {
	Name() string
	VerifyCandidates(ctx context.Context, req Request) (*Result, error)
}

type StaticVerifier struct {
	Enabled bool
}

func (v StaticVerifier) Name() string {
	return "static_safety"
}

func (v StaticVerifier) VerifyCandidates(ctx context.Context, req Request) (*Result, error) {
	enabled := v.Enabled || req.Mode == strategy.ModeBlunderguard || req.Mode == strategy.ModeHybrid
	result := &Result{Enabled: enabled, Used: enabled, Name: v.Name()}
	if !enabled {
		for _, candidate := range req.Candidates {
			result.Candidates = append(result.Candidates, CandidateResult{
				UCI:    candidate.UCI,
				Status: "not_checked",
				Reason: "Verifier disabled for pure mode.",
				Score: strategy.VerifierScore{
					Status: "not_checked",
					Reason: "Verifier disabled for pure mode.",
				},
			})
		}
		return result, nil
	}
	mateInOneMoves := mateInOneMoves(req.Game)
	for _, candidate := range req.Candidates {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		status := "accepted"
		reason := "No immediate static tactical issue found."
		mateRisk := allowsMateInOne(req.Game, candidate.UCI)
		if mateRisk {
			status = "rejected"
			reason = "Move allows an immediate mate-in-one reply."
		}
		if len(mateInOneMoves) > 0 && !contains(mateInOneMoves, candidate.UCI) {
			status = "warning"
			reason = "A mating move is available; this candidate does not play it."
		}
		result.Candidates = append(result.Candidates, CandidateResult{
			UCI:      candidate.UCI,
			Status:   status,
			Reason:   reason,
			MateRisk: mateRisk,
			Score: strategy.VerifierScore{
				Status:   status,
				MateRisk: mateRisk,
				Reason:   reason,
			},
		})
	}
	return result, nil
}

type LegalOnlyVerifier struct{}

func (v LegalOnlyVerifier) Name() string {
	return "legal_only"
}

func (v LegalOnlyVerifier) VerifyCandidates(ctx context.Context, req Request) (*Result, error) {
	result := &Result{Enabled: false, Used: false, Name: v.Name()}
	for _, candidate := range req.Candidates {
		result.Candidates = append(result.Candidates, CandidateResult{
			UCI:    candidate.UCI,
			Status: "not_checked",
			Reason: "Only deterministic legality filtering was applied.",
			Score: strategy.VerifierScore{
				Status: "not_checked",
				Reason: "Only deterministic legality filtering was applied.",
			},
		})
	}
	return result, nil
}

func mateInOneMoves(game *chesscore.Game) []string {
	if game == nil {
		return nil
	}
	var out []string
	for _, mv := range game.LegalMoves() {
		clone := game.Clone()
		if _, err := clone.ApplyUCI(mv.UCI); err != nil {
			continue
		}
		if clone.Outcome().Status == "checkmate" {
			out = append(out, mv.UCI)
		}
	}
	return out
}

func allowsMateInOne(game *chesscore.Game, moveUCI string) bool {
	if game == nil {
		return false
	}
	after := game.Clone()
	if _, err := after.ApplyUCI(moveUCI); err != nil {
		return true
	}
	for _, reply := range after.LegalMoves() {
		next := after.Clone()
		if _, err := next.ApplyUCI(reply.UCI); err != nil {
			continue
		}
		if next.Outcome().Status == "checkmate" {
			return true
		}
	}
	return false
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
