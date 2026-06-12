package chesscore

import (
	"fmt"
	"io"
	"strings"

	chess "github.com/corentings/chess/v2"
	"github.com/google/uuid"
)

type Game struct {
	id string
	g  *chess.Game
}

func NewGame() *Game {
	return &Game{id: uuid.NewString(), g: chess.NewGame()}
}

func FromFEN(fen string) (*Game, error) {
	opt, err := chess.FEN(strings.TrimSpace(fen))
	if err != nil {
		return nil, err
	}
	return &Game{id: uuid.NewString(), g: chess.NewGame(opt)}, nil
}

func FromPGN(r io.Reader) (*Game, error) {
	opt, err := chess.PGN(r)
	if err != nil {
		return nil, err
	}
	return &Game{id: uuid.NewString(), g: chess.NewGame(opt)}, nil
}

func (g *Game) ID() string {
	return g.id
}

func (g *Game) Clone() *Game {
	return &Game{id: g.id, g: g.g.Clone()}
}

func (g *Game) FEN() string {
	return g.g.FEN()
}

func (g *Game) PGN() string {
	return g.g.String()
}

func (g *Game) Ply() int {
	return len(g.g.Moves())
}

func (g *Game) SideToMove() string {
	return colorName(g.g.Position().Turn())
}

func (g *Game) Outcome() Outcome {
	return outcomeDTO(g.g.Outcome(), g.g.Method())
}

func (g *Game) LegalMoves() []LegalMove {
	pos := g.g.Position()
	moves := g.g.ValidMoves()
	out := make([]LegalMove, 0, len(moves))
	for i := range moves {
		m := moves[i]
		out = append(out, moveDTO(pos, &m))
	}
	return out
}

func (g *Game) ApplyUCI(moveUCI string) (MoveRecord, error) {
	moveUCI = strings.TrimSpace(moveUCI)
	if moveUCI == "" {
		return MoveRecord{}, fmt.Errorf("empty move")
	}
	pos := g.g.Position()
	move, err := chess.UCINotation{}.Decode(pos, moveUCI)
	if err != nil {
		return MoveRecord{}, err
	}
	san := chess.AlgebraicNotation{}.Encode(pos, move)
	uci := chess.UCINotation{}.Encode(pos, move)
	if !g.isLegalUCIAtPosition(uci) {
		return MoveRecord{}, fmt.Errorf("illegal move %s", moveUCI)
	}
	if err := g.g.Move(move, nil); err != nil {
		return MoveRecord{}, err
	}
	return MoveRecord{
		Ply:      len(g.g.Moves()),
		UCI:      uci,
		SAN:      san,
		FENAfter: g.g.FEN(),
	}, nil
}

func (g *Game) IsLegalUCI(moveUCI string) bool {
	return g.isLegalUCIAtPosition(moveUCI)
}

func (g *Game) isLegalUCIAtPosition(moveUCI string) bool {
	for _, mv := range g.LegalMoves() {
		if mv.UCI == moveUCI {
			return true
		}
	}
	return false
}

func (g *Game) Snapshot() Snapshot {
	return Snapshot{
		GameID:      g.id,
		FEN:         g.FEN(),
		PGN:         g.PGN(),
		SideToMove:  g.SideToMove(),
		LegalMoves:  g.LegalMoves(),
		MoveHistory: g.MoveHistory(),
		Outcome:     g.Outcome(),
		Ply:         g.Ply(),
		Board:       g.BoardMap(),
	}
}

func (g *Game) MoveHistory() []MoveRecord {
	moves := g.g.Moves()
	positions := g.g.Positions()
	history := make([]MoveRecord, 0, len(moves))
	for i, mv := range moves {
		before := g.g.Position()
		if i < len(positions) {
			before = positions[i]
		}
		fenAfter := ""
		if mv.Position() != nil {
			fenAfter = mv.Position().String()
		}
		history = append(history, MoveRecord{
			Ply:      i + 1,
			UCI:      chess.UCINotation{}.Encode(before, mv),
			SAN:      chess.AlgebraicNotation{}.Encode(before, mv),
			FENAfter: fenAfter,
			Comment:  mv.Comments(),
		})
	}
	return history
}

func (g *Game) BoardMap() map[string]string {
	out := make(map[string]string, 32)
	for sq, piece := range g.g.Position().Board().SquareMap() {
		if piece == chess.NoPiece {
			continue
		}
		out[sq.String()] = piece.String()
	}
	return out
}

func (g *Game) AnnotateLastMove(comment string) {
	moves := g.g.Moves()
	if len(moves) == 0 {
		return
	}
	moves[len(moves)-1].SetComment(sanitizePGNComment(comment))
}

func (g *Game) Undo(plies int) int {
	if plies <= 0 {
		return 0
	}
	undone := 0
	for undone < plies && g.g.GoBack() {
		undone++
	}
	return undone
}

func (g *Game) NormalizeMove(raw string) (LegalMove, bool) {
	return NormalizeMove(g.g.Position(), raw, g.LegalMoves())
}

func moveDTO(pos *chess.Position, m *chess.Move) LegalMove {
	uci := chess.UCINotation{}.Encode(pos, m)
	return LegalMove{
		UCI:       uci,
		SAN:       chess.AlgebraicNotation{}.Encode(pos, m),
		LAN:       chess.LongAlgebraicNotation{}.Encode(pos, m),
		From:      m.S1().String(),
		To:        m.S2().String(),
		Promotion: promotionString(m.Promo()),
		Capture:   m.HasTag(chess.Capture) || m.HasTag(chess.EnPassant),
		Check:     m.HasTag(chess.Check),
	}
}

func NormalizeMove(pos *chess.Position, raw string, legal []LegalMove) (LegalMove, bool) {
	cleaned := cleanMove(raw)
	for _, mv := range legal {
		if mv.UCI == cleaned || strings.EqualFold(mv.SAN, cleaned) || strings.EqualFold(mv.LAN, cleaned) {
			return mv, true
		}
	}
	notations := []chess.Notation{
		chess.UCINotation{},
		chess.AlgebraicNotation{},
		chess.LongAlgebraicNotation{},
	}
	for _, notation := range notations {
		move, err := notation.Decode(pos, cleaned)
		if err != nil {
			continue
		}
		uci := chess.UCINotation{}.Encode(pos, move)
		for _, mv := range legal {
			if mv.UCI == uci {
				return mv, true
			}
		}
	}
	return LegalMove{}, false
}

func cleanMove(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "`\"'")
	replacements := []string{"!!", "??", "!?", "?!", "!", "?", "+", "#", " e.p.", "ep"}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r, "")
	}
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "—", "-")
	s = strings.ReplaceAll(s, "-", "")
	return strings.TrimSpace(s)
}

func colorName(c chess.Color) string {
	switch c {
	case chess.White:
		return "white"
	case chess.Black:
		return "black"
	default:
		return "unknown"
	}
}

func outcomeDTO(outcome chess.Outcome, method chess.Method) Outcome {
	if outcome == chess.NoOutcome || outcome == chess.UnknownOutcome {
		return Outcome{Status: "ongoing", Method: method.String()}
	}
	dto := Outcome{Method: method.String()}
	switch method {
	case chess.Checkmate:
		dto.Status = "checkmate"
	case chess.Stalemate, chess.ThreefoldRepetition, chess.FivefoldRepetition, chess.FiftyMoveRule, chess.SeventyFiveMoveRule, chess.InsufficientMaterial, chess.DrawOffer:
		dto.Status = "draw"
	case chess.Resignation:
		dto.Status = "resignation"
	default:
		if outcome == chess.Draw {
			dto.Status = "draw"
		} else {
			dto.Status = "game_over"
		}
	}
	switch outcome {
	case chess.WhiteWon:
		dto.Winner = "white"
	case chess.BlackWon:
		dto.Winner = "black"
	}
	return dto
}

func promotionString(piece chess.PieceType) string {
	switch piece {
	case chess.Queen:
		return "q"
	case chess.Rook:
		return "r"
	case chess.Bishop:
		return "b"
	case chess.Knight:
		return "n"
	default:
		return ""
	}
}

func sanitizePGNComment(comment string) string {
	comment = strings.ReplaceAll(comment, "{", "(")
	comment = strings.ReplaceAll(comment, "}", ")")
	comment = strings.ReplaceAll(comment, "\n", " ")
	comment = strings.TrimSpace(comment)
	if len(comment) > 500 {
		comment = comment[:500]
	}
	return comment
}
