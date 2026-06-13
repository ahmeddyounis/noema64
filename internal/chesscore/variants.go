package chesscore

import (
	"fmt"
	"strings"
)

type Variant string

const (
	VariantStandard Variant = "standard"
	VariantChess960 Variant = "chess960"
	VariantCustom   Variant = "custom"
)

const VariantStartSchemaVersion = "variant-start.v1"
const CustomBoardDefinitionSchemaVersion = "custom-board-definition.v1"

const (
	CastlingModeNone             = "none"
	CastlingModeStandard         = "standard"
	CastlingModeChess960External = "chess960_external"
)

const standardStartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type VariantStart struct {
	SchemaVersion    string                 `json:"schema_version"`
	Variant          Variant                `json:"variant"`
	Seed             int64                  `json:"seed,omitempty"`
	FEN              string                 `json:"fen"`
	RuleSet          string                 `json:"rule_set,omitempty"`
	CastlingEnabled  bool                   `json:"castling_enabled"`
	CastlingMode     string                 `json:"castling_mode,omitempty"`
	CastlingRights   string                 `json:"castling_rights,omitempty"`
	BoardDefinition  *CustomBoardDefinition `json:"board_definition,omitempty"`
	UnsupportedRules []string               `json:"unsupported_rules,omitempty"`
	Notes            []string               `json:"notes,omitempty"`
}

type CustomBoardDefinition struct {
	SchemaVersion string            `json:"schema_version"`
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	InitialFEN    string            `json:"initial_fen"`
	RuleSet       string            `json:"rule_set"`
	BoardWidth    int               `json:"board_width"`
	BoardHeight   int               `json:"board_height"`
	PieceRules    []CustomPieceRule `json:"piece_rules,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type CustomPieceRule struct {
	Symbol        string   `json:"symbol"`
	Name          string   `json:"name"`
	Move          string   `json:"move"`
	LeaperOffsets []string `json:"leaper_offsets,omitempty"`
	Royal         bool     `json:"royal,omitempty"`
	PromotesTo    []string `json:"promotes_to,omitempty"`
}

func StandardStart(fen string) VariantStart {
	fen = strings.TrimSpace(fen)
	if fen == "" {
		fen = standardStartFEN
	}
	return VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantStandard,
		FEN:             fen,
		RuleSet:         "standard",
		CastlingEnabled: true,
		CastlingMode:    CastlingModeStandard,
		CastlingRights:  fenCastlingField(fen),
	}
}

func Chess960Start(seed int64) VariantStart {
	index := seed % 960
	if index < 0 {
		index += 960
	}
	backRank := chess960BackRank(int(index))
	fen := strings.ToLower(backRank) + "/pppppppp/8/8/8/8/PPPPPPPP/" + backRank + " w - - 0 1"
	return VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantChess960,
		Seed:            index,
		FEN:             fen,
		RuleSet:         "chess960",
		CastlingEnabled: true,
		CastlingMode:    CastlingModeChess960External,
		CastlingRights:  "KQkq",
		Notes: []string{
			"Generated from a deterministic Chess960 start index.",
			"Chess960 castling is generated and applied by Noema64's compatibility layer.",
		},
	}
}

func CustomBoardStart(fen string) (VariantStart, error) {
	fen = strings.TrimSpace(fen)
	if fen == "" {
		return VariantStart{}, fmt.Errorf("custom board FEN is required")
	}
	if _, err := FromFEN(fen); err != nil {
		return VariantStart{}, err
	}
	return VariantStart{
		SchemaVersion:   VariantStartSchemaVersion,
		Variant:         VariantCustom,
		FEN:             fen,
		RuleSet:         "standard",
		CastlingEnabled: fenCastlingField(fen) != "-",
		CastlingMode:    customCastlingMode(fen),
		CastlingRights:  fenCastlingField(fen),
		Notes:           []string{"Custom board loaded from validated FEN."},
	}, nil
}

func CustomBoardStartFromDefinition(def CustomBoardDefinition) (VariantStart, error) {
	def, err := ValidateCustomBoardDefinition(def)
	if err != nil {
		return VariantStart{}, err
	}
	start, err := CustomBoardStart(def.InitialFEN)
	if err != nil {
		return VariantStart{}, err
	}
	start.BoardDefinition = &def
	start.RuleSet = def.RuleSet
	if def.RuleSet != "standard" && def.RuleSet != "chess960" {
		start.UnsupportedRules = append(start.UnsupportedRules, "custom_rule_execution")
		start.Notes = append(start.Notes, "Custom rule metadata is persisted for tooling; live play uses validated FEN and standard legal move generation.")
	}
	if len(def.PieceRules) > 0 {
		start.UnsupportedRules = append(start.UnsupportedRules, "custom_piece_move_generation")
	}
	return start, nil
}

func ValidateCustomBoardDefinition(def CustomBoardDefinition) (CustomBoardDefinition, error) {
	if def.SchemaVersion == "" {
		def.SchemaVersion = CustomBoardDefinitionSchemaVersion
	}
	if def.SchemaVersion != CustomBoardDefinitionSchemaVersion {
		return CustomBoardDefinition{}, fmt.Errorf("unsupported custom board schema_version %q", def.SchemaVersion)
	}
	def.ID = strings.TrimSpace(def.ID)
	if def.ID == "" {
		return CustomBoardDefinition{}, fmt.Errorf("custom board id is required")
	}
	def.Name = strings.TrimSpace(def.Name)
	if def.Name == "" {
		def.Name = def.ID
	}
	def.InitialFEN = strings.TrimSpace(def.InitialFEN)
	if def.InitialFEN == "" {
		return CustomBoardDefinition{}, fmt.Errorf("custom board initial_fen is required")
	}
	if _, err := FromFEN(def.InitialFEN); err != nil {
		return CustomBoardDefinition{}, err
	}
	if def.RuleSet == "" {
		def.RuleSet = "standard"
	}
	if def.BoardWidth == 0 {
		def.BoardWidth = 8
	}
	if def.BoardHeight == 0 {
		def.BoardHeight = 8
	}
	if def.BoardWidth != 8 || def.BoardHeight != 8 {
		return CustomBoardDefinition{}, fmt.Errorf("only 8x8 custom boards are playable in this release")
	}
	for _, rule := range def.PieceRules {
		if strings.TrimSpace(rule.Symbol) == "" || strings.TrimSpace(rule.Name) == "" {
			return CustomBoardDefinition{}, fmt.Errorf("custom piece rules require symbol and name")
		}
	}
	return def, nil
}

func NormalizeVariantStart(start VariantStart, fallbackFEN string) VariantStart {
	if strings.TrimSpace(start.FEN) == "" {
		start.FEN = strings.TrimSpace(fallbackFEN)
	}
	if strings.TrimSpace(start.FEN) == "" {
		start.FEN = standardStartFEN
	}
	if start.SchemaVersion == "" {
		start.SchemaVersion = VariantStartSchemaVersion
	}
	switch start.Variant {
	case VariantStandard, VariantChess960, VariantCustom:
	default:
		start.Variant = VariantCustom
	}
	if start.Variant == VariantStandard && start.FEN == standardStartFEN {
		start.CastlingEnabled = true
	}
	if start.RuleSet == "" {
		start.RuleSet = string(start.Variant)
	}
	if start.CastlingMode == "" {
		if start.CastlingEnabled {
			start.CastlingMode = CastlingModeStandard
		} else {
			start.CastlingMode = CastlingModeNone
		}
	}
	if start.CastlingRights == "" {
		start.CastlingRights = fenCastlingField(start.FEN)
	}
	return start
}

func chess960BackRank(index int) string {
	board := [8]byte{}
	remaining := []int{0, 1, 2, 3, 4, 5, 6, 7}
	n := index

	darkSquares := []int{0, 2, 4, 6}
	board[darkSquares[n%4]] = 'B'
	n /= 4
	lightSquares := []int{1, 3, 5, 7}
	board[lightSquares[n%4]] = 'B'
	n /= 4
	remaining = emptySquares(board)

	qIdx := n % len(remaining)
	board[remaining[qIdx]] = 'Q'
	n /= len(remaining)
	remaining = emptySquares(board)

	knightCombos := [10][2]int{
		{0, 1}, {0, 2}, {0, 3}, {0, 4}, {1, 2},
		{1, 3}, {1, 4}, {2, 3}, {2, 4}, {3, 4},
	}
	combo := knightCombos[n%10]
	board[remaining[combo[0]]] = 'N'
	board[remaining[combo[1]]] = 'N'
	remaining = emptySquares(board)

	board[remaining[0]] = 'R'
	board[remaining[1]] = 'K'
	board[remaining[2]] = 'R'
	return string(board[:])
}

func emptySquares(board [8]byte) []int {
	out := make([]int, 0, 8)
	for i, piece := range board {
		if piece == 0 {
			out = append(out, i)
		}
	}
	return out
}

func fenCastlingField(fen string) string {
	fields := strings.Fields(fen)
	if len(fields) < 3 {
		return ""
	}
	return fields[2]
}

func customCastlingMode(fen string) string {
	if fenCastlingField(fen) == "-" {
		return CastlingModeNone
	}
	return CastlingModeStandard
}
