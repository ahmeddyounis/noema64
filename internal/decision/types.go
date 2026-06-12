package decision

import (
	"time"

	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

type Request struct {
	Game           *chesscore.Game
	Memory         strategy.StrategyMemory
	Mode           strategy.EngineMode
	Personality    strategy.Personality
	Provider       providers.Provider
	Verifier       verifier.Verifier
	Model          string
	Temperature    float64
	MaxTokens      int
	MaxCandidates  int
	Timeout        time.Duration
	LogRawPrompts  bool
	LogRawResponse bool
}

type MoveDecision struct {
	SchemaVersion      string                   `json:"schema_version"`
	DecisionID         string                   `json:"decision_id"`
	GameID             string                   `json:"game_id"`
	Ply                int                      `json:"ply"`
	Mode               strategy.EngineMode      `json:"mode"`
	SelectedMove       chesscore.LegalMove      `json:"selected_move"`
	Explanation        string                   `json:"explanation"`
	PositionSummary    string                   `json:"position_summary"`
	PreviousPlanStatus string                   `json:"previous_plan_status"`
	StrategyBefore     strategy.StrategyMemory  `json:"strategy_before"`
	StrategyAfter      strategy.StrategyMemory  `json:"strategy_after"`
	StrategyDiff       strategy.MemoryDiff      `json:"strategy_diff"`
	CandidateMoves     []strategy.CandidateMove `json:"candidate_moves"`
	RepairAttempts     []strategy.RepairAttempt `json:"repair_attempts"`
	VerifierTrace      *verifier.Result         `json:"verifier_trace"`
	FallbackUsed       bool                     `json:"fallback_used"`
	FallbackReason     string                   `json:"fallback_reason,omitempty"`
	Provider           ProviderTrace            `json:"provider"`
	Timing             Timing                   `json:"timing"`
	Assistance         AssistanceTrace          `json:"assistance"`
	FENBefore          string                   `json:"fen_before"`
	LegalMovesCount    int                      `json:"legal_moves_count"`
}

type ProviderTrace struct {
	Name          string       `json:"name"`
	Model         string       `json:"model"`
	PromptVersion string       `json:"prompt_version"`
	ParseStatus   string       `json:"parse_status"`
	RawAvailable  bool         `json:"raw_available"`
	Error         string       `json:"error,omitempty"`
	RawPrompt     *PromptTrace `json:"raw_prompt,omitempty"`
	RawResponse   string       `json:"raw_response,omitempty"`
}

type PromptTrace struct {
	System string `json:"system"`
	User   string `json:"user"`
}

type Timing struct {
	TotalMS    int64 `json:"total_ms"`
	ProviderMS int64 `json:"provider_ms"`
	VerifierMS int64 `json:"verifier_ms"`
	SearchMS   int64 `json:"search_ms"`
	OtherMS    int64 `json:"other_ms"`
}

type AssistanceTrace struct {
	Mode         strategy.EngineMode `json:"mode"`
	VerifierUsed bool                `json:"verifier_used"`
	VerifierName string              `json:"verifier_name,omitempty"`
	SearchUsed   bool                `json:"search_used"`
	SearchName   string              `json:"search_name,omitempty"`
}
