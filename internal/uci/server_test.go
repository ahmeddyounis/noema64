package uci

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedyounis/noema64/internal/storage"
)

func TestUCISmoke(t *testing.T) {
	input := strings.Join([]string{
		"uci",
		"isready",
		"ucinewgame",
		"position startpos moves e2e4 e7e5 g1f3",
		"go movetime 100",
		"quit",
		"",
	}, "\n")
	var out bytes.Buffer
	settings := storage.DefaultSettings()
	settings.Logging.OutputDir = t.TempDir()
	server := NewServer(strings.NewReader(input), &out, &bytes.Buffer{}, settings)
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	text := out.String()
	for _, want := range []string{"id name Noema64", "uciok", "readyok", "bestmove "} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if !validUCILine(line) {
			t.Fatalf("non-UCI stdout line: %q\n%s", line, text)
		}
	}
}

func TestUCITraceFileOption(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	input := strings.Join([]string{
		"uci",
		"setoption name TraceFile value " + tracePath,
		"position startpos moves e2e4 e7e5",
		"go movetime 100",
		"quit",
		"",
	}, "\n")
	var out bytes.Buffer
	server := NewServer(strings.NewReader(input), &out, &bytes.Buffer{}, storage.DefaultSettings())
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "bestmove ") {
		t.Fatalf("missing bestmove:\n%s", out.String())
	}
	if _, err := os.Stat(tracePath); err != nil {
		t.Fatalf("trace file not written: %v", err)
	}
}

func TestUCITraceEnabledOptionDisablesTraceWrites(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	input := strings.Join([]string{
		"uci",
		"setoption name TraceFile value " + tracePath,
		"setoption name TraceEnabled value false",
		"position startpos moves e2e4 e7e5",
		"go movetime 100",
		"quit",
		"",
	}, "\n")
	var out bytes.Buffer
	server := NewServer(strings.NewReader(input), &out, &bytes.Buffer{}, storage.DefaultSettings())
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "bestmove ") {
		t.Fatalf("missing bestmove:\n%s", out.String())
	}
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Fatalf("trace file written with TraceEnabled=false: %v", err)
	}
}

func validUCILine(line string) bool {
	for _, prefix := range []string{"id ", "option ", "uciok", "readyok", "bestmove ", "info "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}
