package experiments

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

const TournamentSchemaVersion = "tournament.v1"

type TournamentEntrant struct {
	ID          string               `json:"id"`
	Mode        strategy.EngineMode  `json:"mode"`
	Personality strategy.Personality `json:"personality,omitempty"`
}

type TournamentSummary struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	Seed          int64              `json:"seed"`
	GamesPerPair  int                `json:"games_per_pair"`
	GamesPlayed   int                `json:"games_played"`
	MaxPlies      int                `json:"max_plies"`
	Results       []TournamentGame   `json:"results"`
	Ratings       []TournamentRating `json:"ratings"`
}

type TournamentGame struct {
	GameIndex int    `json:"game_index"`
	WhiteID   string `json:"white_id"`
	BlackID   string `json:"black_id"`
	Plies     int    `json:"plies"`
	Outcome   string `json:"outcome"`
	WinnerID  string `json:"winner_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

type TournamentRating struct {
	ID     string  `json:"id"`
	Elo    float64 `json:"elo"`
	Games  int     `json:"games"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
}

func (r Runner) Tournament(ctx context.Context, entrants []TournamentEntrant, gamesPerPair int, seed int64) (TournamentSummary, error) {
	if seed == 0 {
		seed = 64
	}
	if gamesPerPair <= 0 {
		gamesPerPair = 2
	}
	if len(entrants) == 0 {
		entrants = DefaultTournamentEntrants()
	}
	if len(entrants) < 2 {
		return TournamentSummary{}, fmt.Errorf("at least two tournament entrants are required")
	}
	ratings := map[string]*TournamentRating{}
	for i := range entrants {
		if entrants[i].ID == "" {
			entrants[i].ID = string(entrants[i].Mode)
		}
		if entrants[i].Mode == "" {
			entrants[i].Mode = strategy.ModePure
		}
		if entrants[i].Personality == "" {
			entrants[i].Personality = strategy.PersonalityBalanced
		}
		ratings[entrants[i].ID] = &TournamentRating{ID: entrants[i].ID, Elo: 1500}
	}
	summary := TournamentSummary{
		SchemaVersion: TournamentSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Seed:          seed,
		GamesPerPair:  gamesPerPair,
		MaxPlies:      maxTournamentPlies(r.MaxPlies),
	}
	gameIndex := 0
	for i := 0; i < len(entrants); i++ {
		for j := i + 1; j < len(entrants); j++ {
			for gameNo := 0; gameNo < gamesPerPair; gameNo++ {
				select {
				case <-ctx.Done():
					summary.Ratings = ratingSlice(ratings)
					return summary, ctx.Err()
				default:
				}
				white := entrants[i]
				black := entrants[j]
				if gameNo%2 == 1 {
					white, black = black, white
				}
				gameIndex++
				result := r.playTournamentGame(ctx, gameIndex, white, black)
				summary.Results = append(summary.Results, result)
				if result.Error == "" {
					summary.GamesPlayed++
					applyTournamentRating(ratings[white.ID], ratings[black.ID], result)
				}
			}
		}
	}
	summary.Ratings = ratingSlice(ratings)
	return summary, nil
}

func DefaultTournamentEntrants() []TournamentEntrant {
	return []TournamentEntrant{
		{ID: "pure", Mode: strategy.ModePure, Personality: strategy.PersonalityBalanced},
		{ID: "blunderguard", Mode: strategy.ModeBlunderguard, Personality: strategy.PersonalityBalanced},
		{ID: "hybrid", Mode: strategy.ModeHybrid, Personality: strategy.PersonalityBalanced},
	}
}

func (r Runner) playTournamentGame(ctx context.Context, index int, white TournamentEntrant, black TournamentEntrant) TournamentGame {
	opts := r.Options
	if opts.Verifier == nil {
		opts.Verifier = verifier.StaticVerifier{}
	}
	e := engine.New(opts)
	state, err := e.NewGame(ctx, engine.NewGameOptions{Side: "white"})
	if err != nil {
		return TournamentGame{GameIndex: index, WhiteID: white.ID, BlackID: black.ID, Error: err.Error()}
	}
	maxPlies := maxTournamentPlies(r.MaxPlies)
	for state.Snapshot.Outcome.Status == "ongoing" && state.Snapshot.Ply < maxPlies {
		entrant := white
		if state.Snapshot.SideToMove == "black" {
			entrant = black
		}
		moveOpts := opts
		moveOpts.Mode = entrant.Mode
		moveOpts.Personality = entrant.Personality
		moveOpts.Verifier = opts.Verifier
		e.SetOptions(moveOpts)
		_, next, err := e.ChooseMove(ctx)
		if err != nil {
			return TournamentGame{GameIndex: index, WhiteID: white.ID, BlackID: black.ID, Plies: state.Snapshot.Ply, Error: err.Error()}
		}
		state = next
	}
	outcome := state.Snapshot.Outcome.Status
	winner := ""
	switch {
	case state.Snapshot.Outcome.Winner == "white":
		winner = white.ID
	case state.Snapshot.Outcome.Winner == "black":
		winner = black.ID
	case outcome == "ongoing":
		outcome = "adjudicated_draw"
		if state.Features.MaterialBalance > 250 {
			outcome = "adjudicated_material"
			winner = white.ID
		} else if state.Features.MaterialBalance < -250 {
			outcome = "adjudicated_material"
			winner = black.ID
		}
	}
	return TournamentGame{
		GameIndex: index,
		WhiteID:   white.ID,
		BlackID:   black.ID,
		Plies:     state.Snapshot.Ply,
		Outcome:   outcome,
		WinnerID:  winner,
	}
}

func applyTournamentRating(white *TournamentRating, black *TournamentRating, result TournamentGame) {
	whiteScore := 0.5
	switch result.WinnerID {
	case white.ID:
		whiteScore = 1
	case black.ID:
		whiteScore = 0
	}
	blackScore := 1 - whiteScore
	updateRating(white, black.Elo, whiteScore)
	updateRating(black, white.Elo, blackScore)
	recordStanding(white, whiteScore)
	recordStanding(black, blackScore)
}

func updateRating(rating *TournamentRating, opponentElo float64, score float64) {
	expected := 1 / (1 + math.Pow(10, (opponentElo-rating.Elo)/400))
	rating.Elo = math.Round((rating.Elo+24*(score-expected))*10) / 10
}

func recordStanding(rating *TournamentRating, score float64) {
	rating.Games++
	switch score {
	case 1:
		rating.Wins++
	case 0:
		rating.Losses++
	default:
		rating.Draws++
	}
}

func ratingSlice(ratings map[string]*TournamentRating) []TournamentRating {
	out := make([]TournamentRating, 0, len(ratings))
	for _, rating := range ratings {
		out = append(out, *rating)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Elo > out[i].Elo || (out[j].Elo == out[i].Elo && out[j].ID < out[i].ID) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func maxTournamentPlies(maxPlies int) int {
	if maxPlies <= 0 {
		return 80
	}
	return maxPlies
}
