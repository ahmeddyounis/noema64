package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

type Options struct {
	Mode          strategy.EngineMode
	Personality   strategy.Personality
	Provider      providers.Provider
	Verifier      verifier.Verifier
	Model         string
	Temperature   float64
	MaxTokens     int
	MaxCandidates int
	MoveTimeout   time.Duration
}

type NewGameOptions struct {
	Side        string               `json:"side"`
	FEN         string               `json:"fen,omitempty"`
	Mode        strategy.EngineMode  `json:"mode,omitempty"`
	Personality strategy.Personality `json:"personality,omitempty"`
}

type GameState struct {
	Snapshot       chesscore.Snapshot      `json:"snapshot"`
	StrategyMemory strategy.StrategyMemory `json:"strategy_memory"`
	LastDecision   *decision.MoveDecision  `json:"last_decision,omitempty"`
}

type Engine struct {
	mu           sync.Mutex
	game         *chesscore.Game
	memory       strategy.StrategyMemory
	lastDecision *decision.MoveDecision
	opts         Options
	cancel       context.CancelFunc
	activeID     string
}

func New(opts Options) *Engine {
	opts = normalizeOptions(opts)
	game := chesscore.NewGame()
	return &Engine{
		game:   game,
		memory: strategy.NewMemory(game.ID(), "white"),
		opts:   opts,
	}
}

func (e *Engine) NewGame(ctx context.Context, opts NewGameOptions) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	var game *chesscore.Game
	var err error
	if opts.FEN != "" {
		game, err = chesscore.FromFEN(opts.FEN)
		if err != nil {
			return nil, err
		}
	} else {
		game = chesscore.NewGame()
	}
	e.game = game
	if opts.Mode != "" {
		e.opts.Mode = opts.Mode
	}
	if opts.Personality != "" {
		e.opts.Personality = opts.Personality
	}
	side := opts.Side
	if side == "" || side == "auto" {
		side = game.SideToMove()
	}
	e.memory = strategy.NewMemory(game.ID(), side)
	e.lastDecision = nil
	return e.stateLocked(), nil
}

func (e *Engine) ApplyUserMove(ctx context.Context, moveUCI string) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.game.Outcome().Status != "ongoing" {
		return nil, fmt.Errorf("game is over")
	}
	if _, err := e.game.ApplyUCI(moveUCI); err != nil {
		return nil, err
	}
	return e.stateLocked(), nil
}

func (e *Engine) LoadPGN(ctx context.Context, pgn string) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	game, err := chesscore.FromPGN(strings.NewReader(pgn))
	if err != nil {
		return nil, err
	}
	e.game = game
	e.memory = strategy.NewMemory(game.ID(), game.SideToMove())
	e.lastDecision = nil
	return e.stateLocked(), nil
}

func (e *Engine) ChooseMove(ctx context.Context) (*decision.MoveDecision, *GameState, error) {
	e.mu.Lock()
	if e.game.Outcome().Status != "ongoing" {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("game is over")
	}
	if e.cancel != nil {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("engine is already thinking")
	}
	rootCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	activeID := fmt.Sprintf("%d", time.Now().UnixNano())
	e.activeID = activeID
	req := decision.Request{
		Game:          e.game.Clone(),
		Memory:        e.memory,
		Mode:          e.opts.Mode,
		Personality:   e.opts.Personality,
		Provider:      e.opts.Provider,
		Verifier:      e.opts.Verifier,
		Model:         e.opts.Model,
		Temperature:   e.opts.Temperature,
		MaxTokens:     e.opts.MaxTokens,
		MaxCandidates: e.opts.MaxCandidates,
		Timeout:       e.opts.MoveTimeout,
	}
	e.mu.Unlock()

	dec, err := decision.ChooseMove(rootCtx, req)

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeID == activeID {
		e.cancel = nil
		e.activeID = ""
	}
	if err != nil {
		return nil, nil, err
	}
	if dec == nil {
		return nil, nil, fmt.Errorf("no move decision")
	}
	if !e.game.IsLegalUCI(dec.SelectedMove.UCI) {
		return nil, nil, fmt.Errorf("selector returned illegal move %s", dec.SelectedMove.UCI)
	}
	if _, err := e.game.ApplyUCI(dec.SelectedMove.UCI); err != nil {
		return nil, nil, err
	}
	e.game.AnnotateLastMove("Plan: " + dec.Explanation)
	e.memory = dec.StrategyAfter
	e.lastDecision = dec
	return dec, e.stateLocked(), nil
}

func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.activeID = ""
	}
	return nil
}

func (e *Engine) State(ctx context.Context) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stateLocked(), nil
}

func (e *Engine) SetOptions(opts Options) {
	e.mu.Lock()
	defer e.mu.Unlock()
	normalized := normalizeOptions(opts)
	if opts.Provider == nil {
		normalized.Provider = e.opts.Provider
	}
	if opts.Verifier == nil {
		normalized.Verifier = e.opts.Verifier
	}
	e.opts = normalized
}

func (e *Engine) LegalMoves(ctx context.Context) ([]chesscore.LegalMove, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.game.LegalMoves(), nil
}

func (e *Engine) ExportPGN(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.game.PGN(), nil
}

func (e *Engine) ExportFEN(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.game.FEN(), nil
}

func (e *Engine) Undo(ctx context.Context, plies int) (*GameState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.game.Undo(plies)
	e.memory = strategy.NewMemory(e.game.ID(), e.game.SideToMove())
	e.lastDecision = nil
	return e.stateLocked(), nil
}

func (e *Engine) stateLocked() *GameState {
	return &GameState{
		Snapshot:       e.game.Snapshot(),
		StrategyMemory: e.memory,
		LastDecision:   e.lastDecision,
	}
}

func normalizeOptions(opts Options) Options {
	if opts.Mode == "" {
		opts.Mode = strategy.ModeBlunderguard
	}
	if opts.Personality == "" {
		opts.Personality = strategy.PersonalityBalanced
	}
	if opts.Provider == nil {
		opts.Provider = providers.MockProvider{}
	}
	if opts.Verifier == nil {
		opts.Verifier = verifier.StaticVerifier{}
	}
	if opts.Model == "" {
		opts.Model = "mock-balanced"
	}
	if opts.MaxCandidates == 0 {
		opts.MaxCandidates = 5
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 1600
	}
	if opts.MoveTimeout == 0 {
		opts.MoveTimeout = 12 * time.Second
	}
	return opts
}
