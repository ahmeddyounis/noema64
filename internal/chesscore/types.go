package chesscore

type LegalMove struct {
	UCI       string `json:"uci"`
	SAN       string `json:"san"`
	LAN       string `json:"lan"`
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion,omitempty"`
	Capture   bool   `json:"capture"`
	Check     bool   `json:"check"`
}

type MoveRecord struct {
	Ply      int    `json:"ply"`
	UCI      string `json:"uci"`
	SAN      string `json:"san"`
	FENAfter string `json:"fen_after"`
	Comment  string `json:"comment,omitempty"`
}

type Outcome struct {
	Status string `json:"status"`
	Winner string `json:"winner,omitempty"`
	Method string `json:"method,omitempty"`
}

type Snapshot struct {
	GameID      string            `json:"game_id"`
	FEN         string            `json:"fen"`
	PGN         string            `json:"pgn"`
	SideToMove  string            `json:"side_to_move"`
	LegalMoves  []LegalMove       `json:"legal_moves"`
	MoveHistory []MoveRecord      `json:"move_history"`
	Outcome     Outcome           `json:"outcome"`
	Ply         int               `json:"ply"`
	Board       map[string]string `json:"board"`
}

type FeatureSummary struct {
	MaterialBalance int      `json:"material_balance"`
	Phase           string   `json:"phase"`
	SideToMove      string   `json:"side_to_move"`
	InCheck         bool     `json:"in_check"`
	LegalMoveCount  int      `json:"legal_move_count"`
	Captures        []string `json:"captures"`
	Checks          []string `json:"checks"`
	Threats         []string `json:"threats"`
	PinnedPieces    []string `json:"pinned_pieces"`
	HangingPieces   []string `json:"hanging_pieces"`
	KingSafety      []string `json:"king_safety"`
}
