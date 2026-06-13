package chesscore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type variantCastlingState struct {
	Rights             string
	WhiteKing          string
	WhiteKingSideRook  string
	WhiteQueenSideRook string
	BlackKing          string
	BlackKingSideRook  string
	BlackQueenSideRook string
}

type castleSpec struct {
	right   byte
	side    string
	king    string
	rook    string
	kingTo  string
	rookTo  string
	san     string
	isWhite bool
}

func (s *variantCastlingState) clone() *variantCastlingState {
	if s == nil {
		return nil
	}
	copy := *s
	return &copy
}

func newVariantCastlingState(fen string, rights string) (*variantCastlingState, error) {
	board := boardFromFEN(fen)
	whiteKing := findPiece(board, 'K', "1")
	blackKing := findPiece(board, 'k', "8")
	if whiteKing == "" || blackKing == "" {
		return nil, fmt.Errorf("chess960 castling requires both kings")
	}
	whiteQueen, whiteKingSide := rookHomes(board, whiteKing, 'R')
	blackQueen, blackKingSide := rookHomes(board, blackKing, 'r')
	if whiteQueen == "" || whiteKingSide == "" || blackQueen == "" || blackKingSide == "" {
		return nil, fmt.Errorf("chess960 castling requires rooks on both sides of each king")
	}
	rights = normalizeCastleRights(rights)
	if rights == "" || rights == "-" {
		rights = "KQkq"
	}
	return &variantCastlingState{
		Rights:             rights,
		WhiteKing:          whiteKing,
		WhiteKingSideRook:  whiteKingSide,
		WhiteQueenSideRook: whiteQueen,
		BlackKing:          blackKing,
		BlackKingSideRook:  blackKingSide,
		BlackQueenSideRook: blackQueen,
	}, nil
}

func (g *Game) variantCastleMoves() []LegalMove {
	if g.castling == nil || g.Outcome().Status != "ongoing" {
		return nil
	}
	board := boardFromFEN(g.FEN())
	side := fenSideToMove(g.FEN())
	out := []LegalMove{}
	for _, spec := range g.castling.specsForSide(side) {
		if !g.castling.castleLegal(board, side, spec) {
			continue
		}
		out = append(out, LegalMove{
			UCI:  spec.king + spec.rook,
			SAN:  spec.san,
			LAN:  spec.san,
			From: spec.king,
			To:   spec.rook,
		})
	}
	return out
}

func (g *Game) applyVariantCastle(moveUCI string, record bool) (MoveRecord, bool, error) {
	if g.castling == nil {
		return MoveRecord{}, false, nil
	}
	board := boardFromFEN(g.FEN())
	side := fenSideToMove(g.FEN())
	for _, spec := range g.castling.specsForSide(side) {
		uci := spec.king + spec.rook
		kingDestinationUCI := spec.king + spec.kingTo
		if moveUCI != uci && moveUCI != kingDestinationUCI && !strings.EqualFold(moveUCI, spec.san) {
			continue
		}
		if !g.castling.castleLegal(board, side, spec) {
			return MoveRecord{}, true, fmt.Errorf("illegal castle %s", moveUCI)
		}
		nextFEN, err := applyCastleToFEN(g.FEN(), board, spec, g.castling.withoutRight(spec.right))
		if err != nil {
			return MoveRecord{}, true, err
		}
		next, err := newChessGameFromFEN(nextFEN)
		if err != nil {
			return MoveRecord{}, true, err
		}
		g.g = next
		g.castling.Rights = g.castling.withoutRight(spec.right)
		rec := MoveRecord{
			Ply:      len(g.appliedUCI) + 1,
			UCI:      uci,
			SAN:      spec.san,
			FENAfter: nextFEN,
		}
		if record {
			g.appliedUCI = append(g.appliedUCI, uci)
			g.history = append(g.history, rec)
		}
		return rec, true, nil
	}
	return MoveRecord{}, false, nil
}

func (g *Game) updateVariantCastlingRights(uci string, boardBefore map[string]rune) {
	if g.castling == nil {
		return
	}
	from, to, ok := SplitUCIMoveSquares(uci)
	if !ok {
		return
	}
	moved := boardBefore[from]
	captured := boardBefore[to]
	remove := ""
	switch moved {
	case 'K':
		remove += "KQ"
	case 'k':
		remove += "kq"
	case 'R':
		if from == g.castling.WhiteKingSideRook {
			remove += "K"
		}
		if from == g.castling.WhiteQueenSideRook {
			remove += "Q"
		}
	case 'r':
		if from == g.castling.BlackKingSideRook {
			remove += "k"
		}
		if from == g.castling.BlackQueenSideRook {
			remove += "q"
		}
	}
	switch captured {
	case 'R':
		if to == g.castling.WhiteKingSideRook {
			remove += "K"
		}
		if to == g.castling.WhiteQueenSideRook {
			remove += "Q"
		}
	case 'r':
		if to == g.castling.BlackKingSideRook {
			remove += "k"
		}
		if to == g.castling.BlackQueenSideRook {
			remove += "q"
		}
	}
	for i := 0; i < len(remove); i++ {
		g.castling.Rights = g.castling.withoutRight(remove[i])
	}
}

func (s *variantCastlingState) specsForSide(side string) []castleSpec {
	if side == "w" {
		return []castleSpec{
			{right: 'K', side: "king", king: s.WhiteKing, rook: s.WhiteKingSideRook, kingTo: "g1", rookTo: "f1", san: "O-O", isWhite: true},
			{right: 'Q', side: "queen", king: s.WhiteKing, rook: s.WhiteQueenSideRook, kingTo: "c1", rookTo: "d1", san: "O-O-O", isWhite: true},
		}
	}
	return []castleSpec{
		{right: 'k', side: "king", king: s.BlackKing, rook: s.BlackKingSideRook, kingTo: "g8", rookTo: "f8", san: "O-O", isWhite: false},
		{right: 'q', side: "queen", king: s.BlackKing, rook: s.BlackQueenSideRook, kingTo: "c8", rookTo: "d8", san: "O-O-O", isWhite: false},
	}
}

func (s *variantCastlingState) castleLegal(board map[string]rune, side string, spec castleSpec) bool {
	if !strings.ContainsRune(s.Rights, rune(spec.right)) {
		return false
	}
	kingPiece := 'K'
	rookPiece := 'R'
	attackerWhite := false
	if !spec.isWhite {
		kingPiece = 'k'
		rookPiece = 'r'
		attackerWhite = true
	}
	if board[spec.king] != kingPiece || board[spec.rook] != rookPiece {
		return false
	}
	if isSquareAttacked(board, spec.king, attackerWhite) {
		return false
	}
	for _, sq := range squaresBetween(spec.king, spec.rook, false) {
		if board[sq] != 0 {
			return false
		}
	}
	for _, sq := range []string{spec.kingTo, spec.rookTo} {
		if sq == spec.king || sq == spec.rook {
			continue
		}
		if board[sq] != 0 {
			return false
		}
	}
	for _, sq := range squaresBetween(spec.king, spec.kingTo, true) {
		if isSquareAttacked(board, sq, attackerWhite) {
			return false
		}
	}
	return side == "w" && spec.isWhite || side == "b" && !spec.isWhite
}

func (s *variantCastlingState) withoutRight(right byte) string {
	rights := strings.ReplaceAll(s.Rights, string(right), "")
	if rights == "" {
		return "-"
	}
	return rights
}

func applyCastleToFEN(fen string, board map[string]rune, spec castleSpec, rights string) (string, error) {
	fields := strings.Fields(fen)
	if len(fields) < 6 {
		return "", fmt.Errorf("invalid FEN")
	}
	kingPiece := 'K'
	rookPiece := 'R'
	if !spec.isWhite {
		kingPiece = 'k'
		rookPiece = 'r'
	}
	delete(board, spec.king)
	delete(board, spec.rook)
	board[spec.kingTo] = kingPiece
	board[spec.rookTo] = rookPiece
	fields[0] = boardToFEN(board)
	if fields[1] == "w" {
		fields[1] = "b"
	} else {
		fields[1] = "w"
		fullMove, _ := strconv.Atoi(fields[5])
		if fullMove <= 0 {
			fullMove = 1
		}
		fields[5] = strconv.Itoa(fullMove + 1)
	}
	fields[2] = "-"
	fields[3] = "-"
	halfMove, _ := strconv.Atoi(fields[4])
	fields[4] = strconv.Itoa(halfMove + 1)
	_ = rights
	return strings.Join(fields, " "), nil
}

func boardFromFEN(fen string) map[string]rune {
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return map[string]rune{}
	}
	board := map[string]rune{}
	rank := 8
	file := 0
	for _, ch := range fields[0] {
		switch {
		case ch == '/':
			rank--
			file = 0
		case ch >= '1' && ch <= '8':
			file += int(ch - '0')
		default:
			if file < 8 && rank >= 1 {
				board[string([]byte{byte('a' + file), byte('0' + rank)})] = ch
			}
			file++
		}
	}
	return board
}

func boardToFEN(board map[string]rune) string {
	ranks := make([]string, 0, 8)
	for rank := 8; rank >= 1; rank-- {
		empty := 0
		var b strings.Builder
		for file := 0; file < 8; file++ {
			sq := string([]byte{byte('a' + file), byte('0' + rank)})
			piece := board[sq]
			if piece == 0 {
				empty++
				continue
			}
			if empty > 0 {
				b.WriteByte(byte('0' + empty))
				empty = 0
			}
			b.WriteRune(piece)
		}
		if empty > 0 {
			b.WriteByte(byte('0' + empty))
		}
		ranks = append(ranks, b.String())
	}
	return strings.Join(ranks, "/")
}

func findPiece(board map[string]rune, piece rune, rank string) string {
	for file := 'a'; file <= 'h'; file++ {
		sq := string(file) + rank
		if board[sq] == piece {
			return sq
		}
	}
	return ""
}

func rookHomes(board map[string]rune, kingSquare string, rook rune) (queenSide string, kingSide string) {
	kingFile := int(kingSquare[0] - 'a')
	rank := kingSquare[1]
	for file := 0; file < 8; file++ {
		sq := string([]byte{byte('a' + file), rank})
		if board[sq] != rook {
			continue
		}
		if file < kingFile {
			queenSide = sq
		}
		if file > kingFile && kingSide == "" {
			kingSide = sq
		}
	}
	return queenSide, kingSide
}

func fenSideToMove(fen string) string {
	fields := strings.Fields(fen)
	if len(fields) < 2 {
		return "w"
	}
	return fields[1]
}

func normalizeCastleRights(rights string) string {
	seen := map[rune]bool{}
	out := []rune{}
	for _, right := range rights {
		if !strings.ContainsRune("KQkq", right) || seen[right] {
			continue
		}
		seen[right] = true
		out = append(out, right)
	}
	if len(out) == 0 {
		return "-"
	}
	order := map[rune]int{'K': 0, 'Q': 1, 'k': 2, 'q': 3}
	sort.SliceStable(out, func(i, j int) bool { return order[out[i]] < order[out[j]] })
	return string(out)
}

func squaresBetween(a string, b string, includeDestination bool) []string {
	if len(a) != 2 || len(b) != 2 || a[1] != b[1] {
		return nil
	}
	start := int(a[0] - 'a')
	end := int(b[0] - 'a')
	if start == end {
		if includeDestination {
			return []string{b}
		}
		return nil
	}
	step := 1
	if end < start {
		step = -1
	}
	out := []string{}
	for file := start + step; file != end; file += step {
		out = append(out, string([]byte{byte('a' + file), a[1]}))
	}
	if includeDestination {
		out = append(out, b)
	}
	return out
}

func isSquareAttacked(board map[string]rune, target string, byWhite bool) bool {
	tf, tr, ok := squareCoords(target)
	if !ok {
		return false
	}
	if byWhite {
		for _, delta := range [][2]int{{-1, -1}, {1, -1}} {
			if board[squareName(tf+delta[0], tr+delta[1])] == 'P' {
				return true
			}
		}
	} else {
		for _, delta := range [][2]int{{-1, 1}, {1, 1}} {
			if board[squareName(tf+delta[0], tr+delta[1])] == 'p' {
				return true
			}
		}
	}
	knight := 'n'
	king := 'k'
	bishops := "bq"
	rooks := "rq"
	if byWhite {
		knight = 'N'
		king = 'K'
		bishops = "BQ"
		rooks = "RQ"
	}
	for _, delta := range [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}} {
		if board[squareName(tf+delta[0], tr+delta[1])] == knight {
			return true
		}
	}
	for _, delta := range [][2]int{{1, 1}, {1, 0}, {1, -1}, {0, -1}, {-1, -1}, {-1, 0}, {-1, 1}, {0, 1}} {
		if board[squareName(tf+delta[0], tr+delta[1])] == king {
			return true
		}
	}
	for _, delta := range [][2]int{{1, 1}, {1, -1}, {-1, -1}, {-1, 1}} {
		if rayAttacked(board, tf, tr, delta[0], delta[1], bishops) {
			return true
		}
	}
	for _, delta := range [][2]int{{1, 0}, {0, -1}, {-1, 0}, {0, 1}} {
		if rayAttacked(board, tf, tr, delta[0], delta[1], rooks) {
			return true
		}
	}
	return false
}

func rayAttacked(board map[string]rune, file int, rank int, df int, dr int, attackers string) bool {
	for {
		file += df
		rank += dr
		sq := squareName(file, rank)
		if sq == "" {
			return false
		}
		piece := board[sq]
		if piece == 0 {
			continue
		}
		return strings.ContainsRune(attackers, piece)
	}
}

func squareCoords(square string) (int, int, bool) {
	if len(square) != 2 || square[0] < 'a' || square[0] > 'h' || square[1] < '1' || square[1] > '8' {
		return 0, 0, false
	}
	return int(square[0] - 'a'), int(square[1] - '1'), true
}

func squareName(file int, rank int) string {
	if file < 0 || file > 7 || rank < 0 || rank > 7 {
		return ""
	}
	return string([]byte{byte('a' + file), byte('1' + rank)})
}

func pieceColor(piece rune) string {
	switch {
	case piece == 0:
		return ""
	case unicode.IsUpper(piece):
		return "white"
	default:
		return "black"
	}
}
