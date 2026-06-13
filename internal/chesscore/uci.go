package chesscore

import (
	"strings"
	"unicode"
)

func SplitUCIMoveSquares(uci string) (string, string, bool) {
	uci = strings.TrimSpace(uci)
	from, next, ok := readUCISquare(uci, 0)
	if !ok {
		return "", "", false
	}
	to, next, ok := readUCISquare(uci, next)
	if !ok {
		return "", "", false
	}
	for _, r := range uci[next:] {
		if !unicode.IsLetter(r) {
			return "", "", false
		}
	}
	return from, to, true
}

func readUCISquare(uci string, start int) (string, int, bool) {
	if start >= len(uci) || !isUCIFile(rune(uci[start])) {
		return "", start, false
	}
	index := start + 1
	for index < len(uci) && unicode.IsDigit(rune(uci[index])) {
		index++
	}
	if index == start+1 {
		return "", start, false
	}
	return uci[start:index], index, true
}

func isUCIFile(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
