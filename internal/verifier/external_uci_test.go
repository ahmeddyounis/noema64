package verifier

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestExternalUCIHealthCheck(t *testing.T) {
	enginePath := buildFakeUCIEngine(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := (ExternalUCI{Path: enginePath}).HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestExternalUCIHealthCheckDoesNotHangWhenEngineIgnoresQuit(t *testing.T) {
	enginePath := buildFakeUCIEngine(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	if err := (ExternalUCI{Path: enginePath}).HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("HealthCheck cleanup took %s, want bounded cleanup", elapsed)
	}
}

func TestExternalUCIVerifyCandidatesRejectsHighLoss(t *testing.T) {
	enginePath := buildFakeScoringUCIEngine(t)
	result, err := (ExternalUCI{Path: enginePath, MoveTimeMS: 10, MaxCentipawnLoss: 100}).VerifyCandidates(context.Background(), Request{
		FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		Candidates: []strategy.CandidateMove{
			{UCI: "e2e4"},
			{UCI: "d2d4"},
		},
		Mode: strategy.ModeBlunderguard,
	})
	if err != nil {
		t.Fatalf("verify candidates: %v", err)
	}
	byMove := map[string]CandidateResult{}
	for _, item := range result.Candidates {
		byMove[item.UCI] = item
	}
	if byMove["e2e4"].Status != "accepted" {
		t.Fatalf("best candidate = %+v, want accepted", byMove["e2e4"])
	}
	if byMove["d2d4"].Status != "rejected" || byMove["d2d4"].Score.CentipawnLoss != 300 {
		t.Fatalf("inferior candidate = %+v, want rejected with 300cp loss", byMove["d2d4"])
	}
}

func buildFakeUCIEngine(t *testing.T, ignoreQuit bool) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable for fake UCI engine build")
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	enginePath := testExecutablePath(dir, "fake-uci")
	quitAction := "return"
	if ignoreQuit {
		quitAction = "continue"
	}
	source := `package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "uci":
			fmt.Println("id name fake-uci")
			fmt.Println("uciok")
		case "isready":
			fmt.Println("readyok")
		case "quit":
			` + quitAction + `
		}
	}
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write fake engine source: %v", err)
	}
	cmd := exec.Command(goTool, "build", "-o", enginePath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake engine: %v\n%s", err, out)
	}
	warmTestExecutable(t, enginePath)
	return enginePath
}

func buildFakeScoringUCIEngine(t *testing.T) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable for fake UCI engine build")
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	enginePath := testExecutablePath(dir, "fake-scoring-uci")
	source := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "uci":
			fmt.Println("id name fake-scoring-uci")
			fmt.Println("uciok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "go "):
			score := 100
			best := "e2e4"
			if strings.Contains(line, "d2d4") {
				score = -200
				best = "d2d4"
			}
			fmt.Printf("info depth 1 score cp %d nodes 1\n", score)
			fmt.Println("bestmove " + best)
		case line == "quit":
			return
		}
	}
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write fake scoring engine source: %v", err)
	}
	cmd := exec.Command(goTool, "build", "-o", enginePath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake scoring engine: %v\n%s", err, out)
	}
	warmTestExecutable(t, enginePath)
	return enginePath
}

func warmTestExecutable(t *testing.T, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = strings.NewReader("")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("warm test executable %s: %v\n%s", path, err, out)
	}
}

func testExecutablePath(dir string, name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
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
