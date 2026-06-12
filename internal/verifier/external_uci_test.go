package verifier

import "testing"

func TestParseUCIScore(t *testing.T) {
	tests := []struct {
		line string
		want int
		ok   bool
	}{
		{"info depth 8 score cp 34 nodes 12", 34, true},
		{"info depth 8 score cp -125 nodes 12", -125, true},
		{"info depth 8 score mate 3 nodes 12", 99997, true},
		{"info depth 8 score mate -2 nodes 12", -99998, true},
		{"info depth 8 nodes 12", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseUCIScore(tt.line)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("parseUCIScore(%q) = %d,%t want %d,%t", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}
