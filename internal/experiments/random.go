package experiments

import (
	"context"
	"math/rand"
	"time"

	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

type Runner struct {
	Options engine.Options
}

type Summary struct {
	SchemaVersion     string        `json:"schema_version"`
	GamesRequested    int           `json:"games_requested"`
	GamesCompleted    int           `json:"games_completed"`
	IllegalFinalMoves int           `json:"illegal_final_moves"`
	EngineErrors      int           `json:"engine_errors"`
	FallbacksUsed     int           `json:"fallbacks_used"`
	TotalPlies        int           `json:"total_plies"`
	DurationMS        int64         `json:"duration_ms"`
	Results           []GameSummary `json:"results"`
}

type GameSummary struct {
	GameIndex     int    `json:"game_index"`
	Plies         int    `json:"plies"`
	Outcome       string `json:"outcome"`
	FallbacksUsed int    `json:"fallbacks_used"`
	EngineError   string `json:"engine_error,omitempty"`
}

func (r Runner) RandomLegalBenchmark(ctx context.Context, games int, seed int64) (Summary, error) {
	if seed == 0 {
		seed = 64
	}
	rng := rand.New(rand.NewSource(seed))
	start := time.Now()
	summary := Summary{SchemaVersion: "1.0", GamesRequested: games}
	for i := 0; i < games; i++ {
		result, err := r.playOne(ctx, i+1, rng)
		if err != nil {
			result.EngineError = err.Error()
			summary.EngineErrors++
		}
		if result.EngineError == "" {
			summary.GamesCompleted++
		}
		summary.FallbacksUsed += result.FallbacksUsed
		summary.TotalPlies += result.Plies
		summary.Results = append(summary.Results, result)
		select {
		case <-ctx.Done():
			summary.DurationMS = time.Since(start).Milliseconds()
			return summary, ctx.Err()
		default:
		}
	}
	summary.DurationMS = time.Since(start).Milliseconds()
	return summary, nil
}

func (r Runner) playOne(ctx context.Context, index int, rng *rand.Rand) (GameSummary, error) {
	opts := r.Options
	if opts.Mode == "" {
		opts.Mode = strategy.ModePure
	}
	if opts.Verifier == nil {
		opts.Verifier = verifier.LegalOnlyVerifier{}
	}
	e := engine.New(opts)
	state, err := e.NewGame(ctx, engine.NewGameOptions{Side: "white"})
	if err != nil {
		return GameSummary{GameIndex: index}, err
	}
	result := GameSummary{GameIndex: index}
	for state.Snapshot.Outcome.Status == "ongoing" && state.Snapshot.Ply < 240 {
		dec, next, err := e.ChooseMove(ctx)
		if err != nil {
			return result, err
		}
		if dec.FallbackUsed {
			result.FallbacksUsed++
		}
		state = next
		if state.Snapshot.Outcome.Status != "ongoing" {
			break
		}
		legal := state.Snapshot.LegalMoves
		if len(legal) == 0 {
			break
		}
		mv := legal[rng.Intn(len(legal))]
		state, err = e.ApplyUserMove(ctx, mv.UCI)
		if err != nil {
			return result, err
		}
	}
	result.Plies = state.Snapshot.Ply
	result.Outcome = state.Snapshot.Outcome.Status
	return result, nil
}
