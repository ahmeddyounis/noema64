package experiments

import (
	"context"
	"time"

	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

const PositionSuiteSchemaVersion = "position-suite.v1"

type SuitePosition struct {
	Name string `json:"name"`
	FEN  string `json:"fen"`
}

type PositionSuiteSummary struct {
	SchemaVersion      string                `json:"schema_version"`
	PositionsRequested int                   `json:"positions_requested"`
	PositionsAnalyzed  int                   `json:"positions_analyzed"`
	EngineErrors       int                   `json:"engine_errors"`
	FallbacksUsed      int                   `json:"fallbacks_used"`
	DurationMS         int64                 `json:"duration_ms"`
	Results            []PositionSuiteResult `json:"results"`
}

type PositionSuiteResult struct {
	Index          int                 `json:"index"`
	Name           string              `json:"name"`
	FEN            string              `json:"fen"`
	SideToMove     string              `json:"side_to_move,omitempty"`
	Mode           strategy.EngineMode `json:"mode"`
	SelectedMove   string              `json:"selected_move,omitempty"`
	SelectedSAN    string              `json:"selected_san,omitempty"`
	CandidateCount int                 `json:"candidate_count,omitempty"`
	FallbackUsed   bool                `json:"fallback_used"`
	Provider       string              `json:"provider,omitempty"`
	Model          string              `json:"model,omitempty"`
	DurationMS     int64               `json:"duration_ms"`
	EngineError    string              `json:"engine_error,omitempty"`
}

func DefaultPositionSuite() []SuitePosition {
	return []SuitePosition{
		{Name: "Opening development", FEN: "rnbqkbnr/pppppppp/8/8/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 1"},
		{Name: "King safety", FEN: "r2q1rk1/ppp2ppp/2n2n2/3pp3/3PP3/2P2N2/PP3PPP/RNBQ1RK1 w - - 0 8"},
		{Name: "Tactical tension", FEN: "r1bq1rk1/ppp2ppp/2n2n2/3pp3/3PP3/2P2N2/PP1N1PPP/R1BQ1RK1 b - - 2 8"},
		{Name: "Endgame conversion", FEN: "8/5pk1/6p1/4P3/5P2/6K1/8/8 w - - 0 42"},
	}
}

func (r Runner) PositionSuite(ctx context.Context, positions []SuitePosition) (PositionSuiteSummary, error) {
	if len(positions) == 0 {
		positions = DefaultPositionSuite()
	}
	start := time.Now()
	summary := PositionSuiteSummary{
		SchemaVersion:      PositionSuiteSchemaVersion,
		PositionsRequested: len(positions),
	}
	for i, position := range positions {
		select {
		case <-ctx.Done():
			summary.DurationMS = time.Since(start).Milliseconds()
			return summary, ctx.Err()
		default:
		}
		result := r.runPosition(ctx, i+1, position)
		if result.EngineError != "" {
			summary.EngineErrors++
		} else {
			summary.PositionsAnalyzed++
		}
		if result.FallbackUsed {
			summary.FallbacksUsed++
		}
		summary.Results = append(summary.Results, result)
	}
	summary.DurationMS = time.Since(start).Milliseconds()
	return summary, nil
}

func (r Runner) runPosition(ctx context.Context, index int, position SuitePosition) PositionSuiteResult {
	start := time.Now()
	opts := r.Options
	if opts.Mode == "" {
		opts.Mode = strategy.ModeBlunderguard
	}
	if opts.Verifier == nil {
		opts.Verifier = verifier.StaticVerifier{}
	}
	e := engine.New(opts)
	state, err := e.NewGame(ctx, engine.NewGameOptions{Side: "auto", FEN: position.FEN, Mode: opts.Mode, Personality: opts.Personality})
	result := PositionSuiteResult{
		Index:      index,
		Name:       position.Name,
		FEN:        position.FEN,
		Mode:       opts.Mode,
		DurationMS: time.Since(start).Milliseconds(),
	}
	if err != nil {
		result.EngineError = err.Error()
		return result
	}
	result.SideToMove = state.Snapshot.SideToMove
	dec, _, err := e.ChooseMove(ctx)
	result.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		result.EngineError = err.Error()
		return result
	}
	result.SelectedMove = dec.SelectedMove.UCI
	result.SelectedSAN = dec.SelectedMove.SAN
	result.CandidateCount = len(dec.CandidateMoves)
	result.FallbackUsed = dec.FallbackUsed
	result.Provider = dec.Provider.Name
	result.Model = dec.Provider.Model
	return result
}
