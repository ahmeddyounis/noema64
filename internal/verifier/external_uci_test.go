package verifier

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestExternalUCIHealthCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell wrapper")
	}
	enginePath := filepath.Join(t.TempDir(), "fake-uci")
	script := `#!/bin/sh
while IFS= read -r line; do
	case "$line" in
		uci)
			printf 'id name fake-uci\n'
			printf 'uciok\n'
			;;
		isready)
			printf 'readyok\n'
			;;
		quit)
			exit 0
			;;
	esac
done
`
	if err := os.WriteFile(enginePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake engine: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := (ExternalUCI{Path: enginePath}).HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

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
