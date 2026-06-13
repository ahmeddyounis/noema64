package chesscore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type customGame struct {
	def      CustomBoardDefinition
	board    map[string]rune
	side     string
	halfmove int
	fullmove int
	terminal Outcome
}

type customDelta struct {
	file int
	rank int
}

type customMovement struct {
	leapers []customDelta
	sliders []customDelta
}

func newCustomGameFromDefinition(def CustomBoardDefinition) (*customGame, error) {
	def, err := ValidateCustomBoardDefinition(def)
	if err != nil {
		return nil, err
	}
	board, side, halfmove, fullmove, err := parseCustomFEN(def.InitialFEN, def.BoardWidth, def.BoardHeight)
	if err != nil {
		return nil, err
	}
	return &customGame{
		def:      def,
		board:    board,
		side:     side,
		halfmove: halfmove,
		fullmove: fullmove,
	}, nil
}

func (c *customGame) clone() *customGame {
	if c == nil {
		return nil
	}
	out := *c
	out.board = map[string]rune{}
	for sq, piece := range c.board {
		out.board[sq] = piece
	}
	return &out
}

func (c *customGame) fen() string {
	rows := make([]string, 0, c.def.BoardHeight)
	for rank := c.def.BoardHeight; rank >= 1; rank-- {
		var row strings.Builder
		empty := 0
		for file := 0; file < c.def.BoardWidth; file++ {
			piece := c.board[customSquareName(file, rank)]
			if piece == 0 {
				empty++
				continue
			}
			if empty > 0 {
				row.WriteString(strconv.Itoa(empty))
				empty = 0
			}
			row.WriteRune(piece)
		}
		if empty > 0 {
			row.WriteString(strconv.Itoa(empty))
		}
		rows = append(rows, row.String())
	}
	if c.fullmove <= 0 {
		c.fullmove = 1
	}
	return fmt.Sprintf("%s %s - - %d %d", strings.Join(rows, "/"), c.side, c.halfmove, c.fullmove)
}

func (c *customGame) sideName() string {
	if c.side == "b" {
		return "black"
	}
	return "white"
}

func (c *customGame) outcome() Outcome {
	if c.terminal.Status != "" {
		return c.terminal
	}
	return c.computeOutcome()
}

func (c *customGame) resign(side string) error {
	if c.outcome().Status != "ongoing" {
		return fmt.Errorf("game is over")
	}
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "white", "w":
		c.terminal = Outcome{Status: "resignation", Winner: "black", Method: "resignation"}
	case "black", "b":
		c.terminal = Outcome{Status: "resignation", Winner: "white", Method: "resignation"}
	default:
		return fmt.Errorf("invalid side %q", side)
	}
	return nil
}

func (c *customGame) computeOutcome() Outcome {
	whiteRoyals := c.royalSquares("w")
	blackRoyals := c.royalSquares("b")
	if len(whiteRoyals) > 0 && len(blackRoyals) == 0 {
		return Outcome{Status: "checkmate", Winner: "white", Method: "royal_capture"}
	}
	if len(blackRoyals) > 0 && len(whiteRoyals) == 0 {
		return Outcome{Status: "checkmate", Winner: "black", Method: "royal_capture"}
	}
	moves := c.legalMovesNoOutcome()
	if len(moves) > 0 {
		return Outcome{Status: "ongoing"}
	}
	if c.inCheck(c.side) {
		return Outcome{Status: "checkmate", Winner: customSideName(oppositeCustomSide(c.side)), Method: "checkmate"}
	}
	return Outcome{Status: "draw", Method: "stalemate"}
}

func (c *customGame) legalMoves() []LegalMove {
	if c.outcome().Status != "ongoing" {
		return []LegalMove{}
	}
	return c.legalMovesNoOutcome()
}

func (c *customGame) legalMovesNoOutcome() []LegalMove {
	pseudo := c.pseudoMoves(c.side, false)
	out := make([]LegalMove, 0, len(pseudo))
	for _, mv := range pseudo {
		next := c.clone()
		next.applyLegalMove(mv)
		if !next.royalsSafe(c.side) {
			continue
		}
		mv.Check = next.inCheck(next.side)
		if mv.Check && !strings.HasSuffix(mv.SAN, "+") && !strings.HasSuffix(mv.SAN, "#") {
			mv.SAN += "+"
			mv.LAN += "+"
		}
		out = append(out, mv)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UCI == out[j].UCI {
			return out[i].Promotion < out[j].Promotion
		}
		return out[i].UCI < out[j].UCI
	})
	return out
}

func (c *customGame) applyUCI(raw string, ply int) (MoveRecord, error) {
	cleaned := cleanMove(raw)
	for _, mv := range c.legalMoves() {
		if mv.UCI != cleaned && !strings.EqualFold(mv.SAN, cleaned) && !strings.EqualFold(mv.LAN, cleaned) {
			continue
		}
		c.applyLegalMove(mv)
		c.terminal = c.computeOutcome()
		san := mv.SAN
		if c.terminal.Status == "checkmate" && strings.HasSuffix(san, "+") {
			san = strings.TrimSuffix(san, "+") + "#"
		}
		return MoveRecord{
			Ply:      ply,
			UCI:      mv.UCI,
			SAN:      san,
			FENAfter: c.fen(),
		}, nil
	}
	return MoveRecord{}, fmt.Errorf("illegal move %s", raw)
}

func (c *customGame) applyLegalMove(mv LegalMove) {
	piece := c.board[mv.From]
	delete(c.board, mv.From)
	if mv.Promotion != "" {
		piece = customPieceForSide([]rune(strings.ToUpper(mv.Promotion))[0], c.side)
	}
	capture := c.board[mv.To] != 0
	c.board[mv.To] = piece
	if capture || strings.EqualFold(customPieceSymbol(piece), "P") {
		c.halfmove = 0
	} else {
		c.halfmove++
	}
	if c.side == "b" {
		c.fullmove++
	}
	c.side = oppositeCustomSide(c.side)
}

func (c *customGame) boardMap() map[string]string {
	out := make(map[string]string, len(c.board))
	for sq, piece := range c.board {
		if piece == 0 {
			continue
		}
		out[sq] = customPieceDisplay(piece)
	}
	return out
}

func (c *customGame) normalizeMove(raw string) (LegalMove, bool) {
	cleaned := cleanMove(raw)
	for _, mv := range c.legalMoves() {
		if mv.UCI == cleaned || strings.EqualFold(mv.SAN, cleaned) || strings.EqualFold(mv.LAN, cleaned) {
			return mv, true
		}
	}
	return LegalMove{}, false
}

func (c *customGame) features(ply int) FeatureSummary {
	legal := c.legalMoves()
	features := FeatureSummary{
		MaterialBalance: c.materialBalance(),
		Phase:           c.phase(ply),
		SideToMove:      c.sideName(),
		InCheck:         c.inCheck(c.side),
		LegalMoveCount:  len(legal),
	}
	for _, mv := range legal {
		if mv.Capture {
			features.Captures = append(features.Captures, mv.UCI)
			features.Threats = append(features.Threats, fmt.Sprintf("%s captures on %s", mv.UCI, mv.To))
		}
		if mv.Check {
			features.Checks = append(features.Checks, mv.UCI)
			features.Threats = append(features.Threats, fmt.Sprintf("%s gives check", mv.UCI))
		}
	}
	features.KingSafety = c.kingSafetyFlags(legal)
	features.HangingPieces = c.hangingPieces(c.side)
	sort.Strings(features.Captures)
	sort.Strings(features.Checks)
	sort.Strings(features.Threats)
	sort.Strings(features.KingSafety)
	sort.Strings(features.HangingPieces)
	return features
}

func (c *customGame) materialBalance() int {
	score := 0
	for _, piece := range c.board {
		value := c.pieceValue(piece)
		if customPieceSide(piece) == "w" {
			score += value
		} else {
			score -= value
		}
	}
	return score
}

func (c *customGame) phase(ply int) string {
	pieceCount := len(c.board)
	majorCount := 0
	for _, piece := range c.board {
		if c.pieceValue(piece) >= 800 {
			majorCount++
		}
	}
	switch {
	case pieceCount <= 10:
		return "endgame"
	case majorCount < 2 || ply > 20:
		return "middlegame"
	default:
		return "opening"
	}
}

func (c *customGame) kingSafetyFlags(legal []LegalMove) []string {
	if !c.inCheck(c.side) {
		return nil
	}
	flags := []string{"in_check"}
	royals := map[string]bool{}
	for _, sq := range c.royalSquares(c.side) {
		royals[sq] = true
	}
	royalMoves := 0
	for _, mv := range legal {
		if royals[mv.From] {
			royalMoves++
		}
	}
	if royalMoves <= 1 {
		flags = append(flags, fmt.Sprintf("limited_royal_mobility:%d", royalMoves))
	}
	return flags
}

func (c *customGame) hangingPieces(side string) []string {
	out := []string{}
	for sq, piece := range c.board {
		if customPieceSide(piece) != side || c.isRoyal(piece) {
			continue
		}
		if !c.squareAttacked(sq, oppositeCustomSide(side)) {
			continue
		}
		if c.squareAttacked(sq, side) {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s attacked", sq, customPieceSymbol(piece)))
	}
	return out
}

func (c *customGame) pseudoMoves(side string, attacksOnly bool) []LegalMove {
	out := []LegalMove{}
	for from, piece := range c.board {
		if customPieceSide(piece) != side {
			continue
		}
		if strings.EqualFold(customPieceSymbol(piece), "P") {
			out = append(out, c.pawnMoves(from, piece, attacksOnly)...)
			continue
		}
		rule := c.ruleForPiece(piece)
		movement := c.movementForRule(rule)
		out = append(out, c.leaperMoves(from, piece, movement.leapers, attacksOnly)...)
		out = append(out, c.sliderMoves(from, piece, movement.sliders, attacksOnly)...)
	}
	return dedupeLegalMoves(out)
}

func (c *customGame) pawnMoves(from string, piece rune, attacksOnly bool) []LegalMove {
	sq, ok := parseCustomSquare(from)
	if !ok {
		return nil
	}
	side := customPieceSide(piece)
	dir := 1
	startRank := 2
	promotionRank := c.def.BoardHeight
	if side == "b" {
		dir = -1
		startRank = c.def.BoardHeight - 1
		promotionRank = 1
	}
	out := []LegalMove{}
	for _, df := range []int{-1, 1} {
		toFile := sq.file + df
		toRank := sq.rank + dir
		if !c.inBounds(toFile, toRank) {
			continue
		}
		to := customSquareName(toFile, toRank)
		target := c.board[to]
		if attacksOnly {
			out = append(out, c.buildMove(piece, from, to, "", target != 0))
			continue
		}
		if target == 0 || customPieceSide(target) == side {
			continue
		}
		out = append(out, c.addPromotionMoves(piece, from, to, target != 0, toRank == promotionRank)...)
	}
	if attacksOnly {
		return out
	}
	oneRank := sq.rank + dir
	if c.inBounds(sq.file, oneRank) {
		one := customSquareName(sq.file, oneRank)
		if c.board[one] == 0 {
			out = append(out, c.addPromotionMoves(piece, from, one, false, oneRank == promotionRank)...)
			twoRank := sq.rank + 2*dir
			two := customSquareName(sq.file, twoRank)
			if sq.rank == startRank && c.inBounds(sq.file, twoRank) && c.board[two] == 0 {
				out = append(out, c.buildMove(piece, from, two, "", false))
			}
		}
	}
	return out
}

func (c *customGame) addPromotionMoves(piece rune, from string, to string, capture bool, promotion bool) []LegalMove {
	if !promotion {
		return []LegalMove{c.buildMove(piece, from, to, "", capture)}
	}
	promotions := c.ruleForPiece(piece).PromotesTo
	if len(promotions) == 0 {
		promotions = []string{"Q", "R", "B", "N"}
	}
	out := make([]LegalMove, 0, len(promotions))
	for _, promotion := range promotions {
		promotion = strings.ToLower(strings.TrimSpace(promotion))
		if promotion == "" {
			continue
		}
		out = append(out, c.buildMove(piece, from, to, promotion[:1], capture))
	}
	return out
}

func (c *customGame) leaperMoves(from string, piece rune, deltas []customDelta, attacksOnly bool) []LegalMove {
	sq, ok := parseCustomSquare(from)
	if !ok {
		return nil
	}
	side := customPieceSide(piece)
	out := []LegalMove{}
	for _, delta := range deltas {
		toFile := sq.file + delta.file
		toRank := sq.rank + delta.rank
		if !c.inBounds(toFile, toRank) {
			continue
		}
		to := customSquareName(toFile, toRank)
		target := c.board[to]
		if attacksOnly {
			out = append(out, c.buildMove(piece, from, to, "", target != 0 && customPieceSide(target) != side))
			continue
		}
		if target != 0 && customPieceSide(target) == side {
			continue
		}
		out = append(out, c.buildMove(piece, from, to, "", target != 0))
	}
	return out
}

func (c *customGame) sliderMoves(from string, piece rune, deltas []customDelta, attacksOnly bool) []LegalMove {
	sq, ok := parseCustomSquare(from)
	if !ok {
		return nil
	}
	side := customPieceSide(piece)
	out := []LegalMove{}
	for _, delta := range deltas {
		file := sq.file + delta.file
		rank := sq.rank + delta.rank
		for c.inBounds(file, rank) {
			to := customSquareName(file, rank)
			target := c.board[to]
			if attacksOnly {
				out = append(out, c.buildMove(piece, from, to, "", target != 0 && customPieceSide(target) != side))
				if target != 0 {
					break
				}
				file += delta.file
				rank += delta.rank
				continue
			}
			if target != 0 {
				if customPieceSide(target) != side {
					out = append(out, c.buildMove(piece, from, to, "", true))
				}
				break
			}
			out = append(out, c.buildMove(piece, from, to, "", false))
			file += delta.file
			rank += delta.rank
		}
	}
	return out
}

func (c *customGame) buildMove(piece rune, from string, to string, promotion string, capture bool) LegalMove {
	uci := from + to + promotion
	san := c.moveSAN(piece, from, to, promotion, capture)
	lanSep := "-"
	if capture {
		lanSep = "x"
	}
	return LegalMove{
		UCI:       uci,
		SAN:       san,
		LAN:       from + lanSep + to,
		From:      from,
		To:        to,
		Promotion: promotion,
		Capture:   capture,
	}
}

func (c *customGame) moveSAN(piece rune, from string, to string, promotion string, capture bool) string {
	symbol := customPieceSymbol(piece)
	prefix := symbol
	if strings.EqualFold(symbol, "P") {
		prefix = ""
		if capture && len(from) > 0 {
			prefix = from[:1]
		}
	}
	captureMark := ""
	if capture {
		captureMark = "x"
	}
	san := prefix + captureMark + to
	if promotion != "" {
		san += "=" + strings.ToUpper(promotion)
	}
	return san
}

func (c *customGame) inBounds(file int, rank int) bool {
	return file >= 0 && file < c.def.BoardWidth && rank >= 1 && rank <= c.def.BoardHeight
}

func (c *customGame) royalsSafe(side string) bool {
	for _, sq := range c.royalSquares(side) {
		if c.squareAttacked(sq, oppositeCustomSide(side)) {
			return false
		}
	}
	return true
}

func (c *customGame) inCheck(side string) bool {
	for _, sq := range c.royalSquares(side) {
		if c.squareAttacked(sq, oppositeCustomSide(side)) {
			return true
		}
	}
	return false
}

func (c *customGame) squareAttacked(square string, bySide string) bool {
	for _, mv := range c.pseudoMoves(bySide, true) {
		if mv.To == square {
			return true
		}
	}
	return false
}

func (c *customGame) royalSquares(side string) []string {
	out := []string{}
	for sq, piece := range c.board {
		if customPieceSide(piece) != side || !c.isRoyal(piece) {
			continue
		}
		out = append(out, sq)
	}
	sort.Strings(out)
	return out
}

func (c *customGame) isRoyal(piece rune) bool {
	if strings.EqualFold(customPieceSymbol(piece), "K") {
		return true
	}
	return c.ruleForPiece(piece).Royal
}

func (c *customGame) ruleForPiece(piece rune) CustomPieceRule {
	symbol := customPieceSymbol(piece)
	for _, rule := range c.def.PieceRules {
		if strings.EqualFold(strings.TrimSpace(rule.Symbol), symbol) {
			rule.Symbol = symbol
			return rule
		}
	}
	return standardCustomPieceRule(symbol)
}

func standardCustomPieceRule(symbol string) CustomPieceRule {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "K":
		return CustomPieceRule{Symbol: "K", Name: "King", Move: "king", Royal: true}
	case "Q":
		return CustomPieceRule{Symbol: "Q", Name: "Queen", Move: "queen"}
	case "R":
		return CustomPieceRule{Symbol: "R", Name: "Rook", Move: "rook"}
	case "B":
		return CustomPieceRule{Symbol: "B", Name: "Bishop", Move: "bishop"}
	case "N":
		return CustomPieceRule{Symbol: "N", Name: "Knight", Move: "knight"}
	case "P":
		return CustomPieceRule{Symbol: "P", Name: "Pawn", Move: "pawn", PromotesTo: []string{"Q", "R", "B", "N"}}
	default:
		return CustomPieceRule{Symbol: symbol, Name: symbol, Move: "king"}
	}
}

func (c *customGame) movementForRule(rule CustomPieceRule) customMovement {
	movement := customMovement{}
	for _, offset := range rule.LeaperOffsets {
		if delta, ok := parseCustomDelta(offset); ok {
			movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(delta, strings.Contains(offset, "-"))...)
		}
	}
	for _, token := range movementTokens(rule.Move) {
		switch token {
		case "", "pawn":
			continue
		case "king":
			movement.leapers = appendDeltas(movement.leapers, allKingDeltas()...)
		case "queen":
			movement.sliders = appendDeltas(movement.sliders, orthogonalDeltas()...)
			movement.sliders = appendDeltas(movement.sliders, diagonalDeltas()...)
		case "rook", "orthogonal", "slide:orthogonal":
			movement.sliders = appendDeltas(movement.sliders, orthogonalDeltas()...)
		case "bishop", "diagonal", "slide:diagonal":
			movement.sliders = appendDeltas(movement.sliders, diagonalDeltas()...)
		case "knight":
			movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(customDelta{file: 1, rank: 2}, false)...)
		case "archbishop", "cardinal", "princess":
			movement.sliders = appendDeltas(movement.sliders, diagonalDeltas()...)
			movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(customDelta{file: 1, rank: 2}, false)...)
		case "chancellor", "empress", "marshall":
			movement.sliders = appendDeltas(movement.sliders, orthogonalDeltas()...)
			movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(customDelta{file: 1, rank: 2}, false)...)
		case "amazon":
			movement.sliders = appendDeltas(movement.sliders, orthogonalDeltas()...)
			movement.sliders = appendDeltas(movement.sliders, diagonalDeltas()...)
			movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(customDelta{file: 1, rank: 2}, false)...)
		case "camel":
			movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(customDelta{file: 1, rank: 3}, false)...)
		case "wazir":
			movement.leapers = appendDeltas(movement.leapers, customDelta{1, 0}, customDelta{-1, 0}, customDelta{0, 1}, customDelta{0, -1})
		case "ferz":
			movement.leapers = appendDeltas(movement.leapers, customDelta{1, 1}, customDelta{1, -1}, customDelta{-1, 1}, customDelta{-1, -1})
		case "alfil":
			movement.leapers = appendDeltas(movement.leapers, customDelta{2, 2}, customDelta{2, -2}, customDelta{-2, 2}, customDelta{-2, -2})
		case "dabbaba":
			movement.leapers = appendDeltas(movement.leapers, customDelta{2, 0}, customDelta{-2, 0}, customDelta{0, 2}, customDelta{0, -2})
		default:
			if strings.HasPrefix(token, "leaper:") {
				if delta, ok := parseCustomDelta(strings.TrimPrefix(token, "leaper:")); ok {
					movement.leapers = appendDeltas(movement.leapers, expandLeaperDelta(delta, strings.Contains(token, "-"))...)
				}
			}
			if strings.HasPrefix(token, "slide:") {
				if delta, ok := parseCustomDelta(strings.TrimPrefix(token, "slide:")); ok {
					movement.sliders = appendDeltas(movement.sliders, delta)
				}
			}
		}
	}
	if len(movement.leapers) == 0 && len(movement.sliders) == 0 {
		movement.leapers = appendDeltas(movement.leapers, allKingDeltas()...)
	}
	return movement
}

func (c *customGame) pieceValue(piece rune) int {
	switch strings.ToUpper(customPieceSymbol(piece)) {
	case "P":
		return 100
	case "N", "B":
		return 320
	case "R":
		return 500
	case "Q":
		return 900
	case "K":
		return 0
	}
	rule := c.ruleForPiece(piece)
	movement := c.movementForRule(rule)
	return 200 + len(movement.leapers)*25 + len(movement.sliders)*160
}

func parseCustomFEN(fen string, width int, height int) (map[string]rune, string, int, int, error) {
	fields := strings.Fields(strings.TrimSpace(fen))
	if len(fields) == 1 {
		fields = append(fields, "w", "-", "-", "0", "1")
	}
	if len(fields) < 6 {
		return nil, "", 0, 0, fmt.Errorf("custom FEN requires board, side, castling, en passant, halfmove, and fullmove fields")
	}
	side := fields[1]
	if side != "w" && side != "b" {
		return nil, "", 0, 0, fmt.Errorf("custom FEN side must be w or b")
	}
	rows := strings.Split(fields[0], "/")
	if len(rows) != height {
		return nil, "", 0, 0, fmt.Errorf("custom FEN has %d ranks, want %d", len(rows), height)
	}
	board := map[string]rune{}
	for rowIdx, row := range rows {
		rank := height - rowIdx
		file := 0
		for i := 0; i < len(row); {
			r := rune(row[i])
			if unicode.IsDigit(r) {
				j := i + 1
				for j < len(row) && unicode.IsDigit(rune(row[j])) {
					j++
				}
				empty, err := strconv.Atoi(row[i:j])
				if err != nil || empty <= 0 {
					return nil, "", 0, 0, fmt.Errorf("invalid empty count in custom FEN rank %d", rank)
				}
				file += empty
				i = j
				continue
			}
			if !unicode.IsLetter(r) {
				return nil, "", 0, 0, fmt.Errorf("invalid custom piece %q in FEN", string(r))
			}
			if file >= width {
				return nil, "", 0, 0, fmt.Errorf("custom FEN rank %d is wider than %d files", rank, width)
			}
			board[customSquareName(file, rank)] = r
			file++
			i++
		}
		if file != width {
			return nil, "", 0, 0, fmt.Errorf("custom FEN rank %d has %d files, want %d", rank, file, width)
		}
	}
	halfmove, err := strconv.Atoi(fields[4])
	if err != nil || halfmove < 0 {
		return nil, "", 0, 0, fmt.Errorf("invalid custom FEN halfmove clock")
	}
	fullmove, err := strconv.Atoi(fields[5])
	if err != nil || fullmove <= 0 {
		return nil, "", 0, 0, fmt.Errorf("invalid custom FEN fullmove number")
	}
	return board, side, halfmove, fullmove, nil
}

func validateCustomPieceRules(rules []CustomPieceRule) ([]CustomPieceRule, error) {
	out := make([]CustomPieceRule, 0, len(rules))
	seen := map[string]bool{}
	for _, rule := range rules {
		rule.Symbol = strings.ToUpper(strings.TrimSpace(rule.Symbol))
		rule.Name = strings.TrimSpace(rule.Name)
		rule.Move = strings.TrimSpace(rule.Move)
		if rule.Symbol == "" || rule.Name == "" {
			return nil, fmt.Errorf("custom piece rules require symbol and name")
		}
		if len([]rune(rule.Symbol)) != 1 || !unicode.IsLetter([]rune(rule.Symbol)[0]) {
			return nil, fmt.Errorf("custom piece symbol %q must be one letter", rule.Symbol)
		}
		if seen[rule.Symbol] {
			return nil, fmt.Errorf("duplicate custom piece symbol %q", rule.Symbol)
		}
		seen[rule.Symbol] = true
		for _, offset := range rule.LeaperOffsets {
			if _, ok := parseCustomDelta(offset); !ok {
				return nil, fmt.Errorf("invalid leaper offset %q for custom piece %s", offset, rule.Symbol)
			}
		}
		if rule.Move == "" && len(rule.LeaperOffsets) == 0 {
			return nil, fmt.Errorf("custom piece %s requires move or leaper_offsets", rule.Symbol)
		}
		out = append(out, rule)
	}
	return out, nil
}

func movementTokens(move string) []string {
	move = strings.ToLower(strings.TrimSpace(move))
	if move == "" {
		return nil
	}
	fields := strings.FieldsFunc(move, func(r rune) bool {
		return r == '+' || r == '|' || r == ';'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func parseCustomDelta(raw string) (customDelta, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	if len(parts) != 2 {
		return customDelta{}, false
	}
	df, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return customDelta{}, false
	}
	dr, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return customDelta{}, false
	}
	if df == 0 && dr == 0 {
		return customDelta{}, false
	}
	return customDelta{file: df, rank: dr}, true
}

func expandLeaperDelta(delta customDelta, exact bool) []customDelta {
	if exact {
		return []customDelta{delta}
	}
	base := []customDelta{
		{delta.file, delta.rank},
		{delta.file, -delta.rank},
		{-delta.file, delta.rank},
		{-delta.file, -delta.rank},
	}
	if absInt(delta.file) != absInt(delta.rank) {
		base = append(base,
			customDelta{delta.rank, delta.file},
			customDelta{delta.rank, -delta.file},
			customDelta{-delta.rank, delta.file},
			customDelta{-delta.rank, -delta.file},
		)
	}
	return appendDeltas(nil, base...)
}

func appendDeltas(base []customDelta, deltas ...customDelta) []customDelta {
	seen := map[customDelta]bool{}
	for _, delta := range base {
		seen[delta] = true
	}
	for _, delta := range deltas {
		if delta.file == 0 && delta.rank == 0 || seen[delta] {
			continue
		}
		base = append(base, delta)
		seen[delta] = true
	}
	return base
}

func orthogonalDeltas() []customDelta {
	return []customDelta{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
}

func diagonalDeltas() []customDelta {
	return []customDelta{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
}

func allKingDeltas() []customDelta {
	return append(orthogonalDeltas(), diagonalDeltas()...)
}

func dedupeLegalMoves(moves []LegalMove) []LegalMove {
	out := make([]LegalMove, 0, len(moves))
	seen := map[string]bool{}
	for _, mv := range moves {
		key := mv.UCI
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, mv)
	}
	return out
}

type customSquare struct {
	file int
	rank int
}

func parseCustomSquare(square string) (customSquare, bool) {
	if len(square) < 2 {
		return customSquare{}, false
	}
	file := int(square[0] - 'a')
	rank, err := strconv.Atoi(square[1:])
	if file < 0 || file >= 26 || err != nil {
		return customSquare{}, false
	}
	return customSquare{file: file, rank: rank}, true
}

func customSquareName(file int, rank int) string {
	return string(rune('a'+file)) + strconv.Itoa(rank)
}

func customPieceSide(piece rune) string {
	if unicode.IsUpper(piece) {
		return "w"
	}
	return "b"
}

func customPieceSymbol(piece rune) string {
	return strings.ToUpper(string(piece))
}

func customPieceForSide(symbol rune, side string) rune {
	if side == "b" {
		return unicode.ToLower(symbol)
	}
	return unicode.ToUpper(symbol)
}

func oppositeCustomSide(side string) string {
	if side == "b" {
		return "w"
	}
	return "b"
}

func customSideName(side string) string {
	if side == "b" {
		return "black"
	}
	return "white"
}

func customPieceDisplay(piece rune) string {
	switch piece {
	case 'K':
		return "♔"
	case 'Q':
		return "♕"
	case 'R':
		return "♖"
	case 'B':
		return "♗"
	case 'N':
		return "♘"
	case 'P':
		return "♙"
	case 'k':
		return "♚"
	case 'q':
		return "♛"
	case 'r':
		return "♜"
	case 'b':
		return "♝"
	case 'n':
		return "♞"
	case 'p':
		return "♟"
	default:
		return string(piece)
	}
}
