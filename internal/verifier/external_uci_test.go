package verifier

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

func buildFakeUCIEngine(t *testing.T, ignoreQuit bool) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable for fake UCI engine build")
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	enginePath := filepath.Join(dir, "fake-uci")
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

func warmTestExecutable(t *testing.T, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, path).Run(); err != nil {
		t.Fatalf("warm test executable %s: %v", path, err)
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
