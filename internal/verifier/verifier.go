package verifier

import (
	"context"
	"fmt"
	"strings"

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
		} else if len(mateInOneMoves) > 0 && !contains(mateInOneMoves, candidate.UCI) {
			status = "rejected"
			reason = "A mating move is available; this candidate does not play it."
		} else if piece, square, reply, ok := directHighValueLoss(req.Game, candidate.UCI, "queen"); ok {
			status = "rejected"
			reason = fmt.Sprintf("Move allows direct capture of %s on %s by %s.", piece, square, reply)
		} else if piece, square, reply, ok := directHighValueLoss(req.Game, candidate.UCI, "rook"); ok {
			status = "warning"
			reason = fmt.Sprintf("Move allows direct capture of %s on %s by %s.", piece, square, reply)
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

func directHighValueLoss(game *chesscore.Game, moveUCI string, target string) (piece string, square string, reply string, ok bool) {
	if game == nil {
		return "", "", "", false
	}
	movingSide := game.SideToMove()
	after := game.Clone()
	if _, err := after.ApplyUCI(moveUCI); err != nil {
		return "", "", "", false
	}
	if after.Outcome().Status != "ongoing" {
		return "", "", "", false
	}
	board := after.Snapshot().Board
	for _, candidateReply := range after.LegalMoves() {
		if !candidateReply.Capture {
			continue
		}
		captured := board[candidateReply.To]
		if pieceSide(captured) != movingSide || pieceKind(captured) != target {
			continue
		}
		return pieceName(captured), candidateReply.To, candidateReply.UCI, true
	}
	return "", "", "", false
}

func pieceSide(piece string) string {
	if piece == "" {
		return ""
	}
	if strings.Contains("♔♕♖♗♘♙", piece) {
		return "white"
	}
	if strings.Contains("♚♛♜♝♞♟", piece) {
		return "black"
	}
	if piece == strings.ToUpper(piece) {
		return "white"
	}
	return "black"
}

func pieceKind(piece string) string {
	switch strings.ToLower(piece) {
	case "q", "♕", "♛":
		return "queen"
	case "r", "♖", "♜":
		return "rook"
	case "b", "♗", "♝":
		return "bishop"
	case "n", "♘", "♞":
		return "knight"
	case "p", "♙", "♟":
		return "pawn"
	case "k", "♔", "♚":
		return "king"
	default:
		return "piece"
	}
}

func pieceName(piece string) string {
	side := pieceSide(piece)
	kind := pieceKind(piece)
	if side == "" {
		return kind
	}
	return side + " " + kind
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
