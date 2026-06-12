package strategy

import "github.com/ahmedyounis/noema64/internal/chesscore"

const (
	MemorySchemaVersion   = "strategy-memory.v1.2"
	DecisionSchemaVersion = "decision-output.v1.2"
	PromptVersion         = "move_selection/1.0.0"
)

type EngineMode string

const (
	ModePure         EngineMode = "pure"
	ModeBlunderguard EngineMode = "blunderguard"
	ModeHybrid       EngineMode = "hybrid"
	ModeCoach        EngineMode = "coach"
)

type Personality string

const (
	PersonalityBalanced      Personality = "balanced"
	PersonalityAggressive    Personality = "aggressive"
	PersonalityPositional    Personality = "positional"
	PersonalityBeginnerCoach Personality = "beginner_coach"
)

type StrategyMemory struct {
	SchemaVersion      string              `json:"schema_version"`
	MemoryID           string              `json:"memory_id"`
	GameID             string              `json:"game_id"`
	Side               string              `json:"side"`
	Ply                int                 `json:"ply"`
	Phase              string              `json:"phase"`
	Plan               Plan                `json:"plan"`
	Targets            Targets             `json:"targets"`
	PieceImprovement   []PieceImprovement  `json:"piece_improvement"`
	PawnBreaks         []PawnBreak         `json:"pawn_breaks"`
	OpponentModel      OpponentModel       `json:"opponent_model"`
	Commitments        []string            `json:"commitments"`
	RefutationTriggers []RefutationTrigger `json:"refutation_triggers"`
	DoNotPlayPatterns  []DoNotPlayPattern  `json:"do_not_play_patterns"`
	LastUpdate         LastUpdate          `json:"last_update"`
	TacticalWarnings   []string            `json:"tactical_warnings"`
	StyleNotes         []string            `json:"style_notes"`
}

type Plan struct {
	Summary      string  `json:"summary"`
	Status       string  `json:"status"`
	Confidence   float64 `json:"confidence"`
	HorizonMoves int     `json:"horizon_moves"`
}

type Targets struct {
	Squares []string `json:"squares"`
	Pieces  []string `json:"pieces"`
	Pawns   []string `json:"pawns"`
}

type PieceImprovement struct {
	Piece          string   `json:"piece"`
	Problem        string   `json:"problem,omitempty"`
	DesiredSquares []string `json:"desired_squares"`
	Priority       float64  `json:"priority"`
	Reason         string   `json:"reason,omitempty"`
}

type PawnBreak struct {
	MovePattern   string   `json:"move_pattern"`
	Purpose       string   `json:"purpose"`
	Preconditions []string `json:"preconditions"`
	Risk          string   `json:"risk"`
}

type OpponentModel struct {
	LikelyPlan string   `json:"likely_plan"`
	Threats    []string `json:"threats"`
	Confidence float64  `json:"confidence"`
}

type RefutationTrigger struct {
	Condition string `json:"condition"`
	Response  string `json:"response"`
}

type DoNotPlayPattern struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

type LastUpdate struct {
	DecisionID string `json:"decision_id"`
	MovePlayed string `json:"move_played"`
	Summary    string `json:"summary"`
}

type DecisionOutput struct {
	SchemaVersion      string            `json:"schema_version"`
	PreviousPlanStatus string            `json:"previous_plan_status"`
	PositionSummary    string            `json:"position_summary"`
	StrategyUpdate     StrategyUpdate    `json:"strategy_update"`
	CandidateMoves     []CandidateMove   `json:"candidate_moves"`
	DoNotPlay          []DoNotPlay       `json:"do_not_play"`
	NeedsReplanAfter   []string          `json:"needs_replan_after,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type StrategyUpdate struct {
	PlanSummary        string   `json:"plan_summary"`
	Phase              string   `json:"phase"`
	MainTargets        []string `json:"main_targets"`
	PieceImprovement   []string `json:"piece_improvement"`
	PawnBreaks         []string `json:"pawn_breaks"`
	OpponentPlanGuess  string   `json:"opponent_plan_guess"`
	Commitments        []string `json:"commitments"`
	RefutationTriggers []string `json:"refutation_triggers"`
	TacticalWarnings   []string `json:"tactical_warnings"`
	Confidence         float64  `json:"confidence"`
	LastUpdateSummary  string   `json:"last_update_summary"`
}

type CandidateMove struct {
	UCI                string              `json:"uci"`
	SAN                string              `json:"san,omitempty"`
	Purpose            string              `json:"purpose"`
	ExpectedReply      string              `json:"expected_reply,omitempty"`
	Risk               string              `json:"risk,omitempty"`
	LLMConfidence      float64             `json:"confidence"`
	PlanAlignmentScore float64             `json:"plan_alignment_score"`
	VerifierScore      VerifierScore       `json:"verifier_score"`
	FinalScore         float64             `json:"final_score"`
	Rank               int                 `json:"rank"`
	LegalMove          chesscore.LegalMove `json:"legal_move"`
	RepairMethod       string              `json:"repair_method,omitempty"`
}

type VerifierScore struct {
	Status        string `json:"status"`
	CentipawnLoss int    `json:"centipawn_loss,omitempty"`
	MateRisk      bool   `json:"mate_risk"`
	Reason        string `json:"reason"`
}

type DoNotPlay struct {
	UCIOrPattern string `json:"uci_or_pattern"`
	Reason       string `json:"reason"`
}

type RepairAttempt struct {
	Raw        string  `json:"raw"`
	Normalized string  `json:"normalized"`
	Method     string  `json:"method"`
	Legal      bool    `json:"legal"`
	Confidence float64 `json:"confidence"`
}

type MemoryDiff struct {
	PlanBefore      string   `json:"plan_before"`
	PlanAfter       string   `json:"plan_after"`
	ChangedFields   []string `json:"changed_fields"`
	ConfidenceDelta float64  `json:"confidence_delta"`
	Reason          string   `json:"reason"`
}

type StrategyRequest struct {
	GameID           string                   `json:"game_id"`
	FEN              string                   `json:"fen"`
	PGN              string                   `json:"pgn"`
	SideToMove       string                   `json:"side_to_move"`
	MoveNumber       int                      `json:"move_number"`
	LastOpponentMove string                   `json:"last_opponent_move"`
	LegalMoves       []chesscore.LegalMove    `json:"legal_moves"`
	Features         chesscore.FeatureSummary `json:"features"`
	PreviousMemory   StrategyMemory           `json:"previous_memory"`
	Mode             EngineMode               `json:"mode"`
	Personality      Personality              `json:"personality"`
	Clock            map[string]int64         `json:"clock,omitempty"`
}
