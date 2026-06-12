package uci

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/verifier"
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

func TestUCIOptionRangesMatchHandshake(t *testing.T) {
	server := NewServer(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, storage.DefaultSettings())
	if err := server.setOption("setoption name Temperature value 999"); err != nil {
		t.Fatalf("set temperature: %v", err)
	}
	if server.opts.Temperature != 2.0 {
		t.Fatalf("temperature = %v, want 2.0", server.opts.Temperature)
	}
	if err := server.setOption("setoption name Temperature value -25"); err != nil {
		t.Fatalf("set temperature: %v", err)
	}
	if server.opts.Temperature != 0 {
		t.Fatalf("temperature = %v, want 0", server.opts.Temperature)
	}
	if err := server.setOption("setoption name MaxCandidates value 99"); err != nil {
		t.Fatalf("set max candidates: %v", err)
	}
	if server.opts.MaxCandidates != 10 {
		t.Fatalf("max candidates = %d, want 10", server.opts.MaxCandidates)
	}
}

func TestUCIVerifierEnabledControlsExternalPath(t *testing.T) {
	server := NewServer(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, storage.DefaultSettings())
	if err := server.setOption("setoption name VerifierPath value /usr/bin/stockfish"); err != nil {
		t.Fatalf("set verifier path: %v", err)
	}
	if _, ok := server.opts.Verifier.(verifier.ExternalUCI); ok {
		t.Fatal("verifier path enabled external verifier without VerifierEnabled=true")
	}
	if err := server.setOption("setoption name VerifierEnabled value true"); err != nil {
		t.Fatalf("enable verifier: %v", err)
	}
	if _, ok := server.opts.Verifier.(verifier.ExternalUCI); !ok {
		t.Fatalf("enabled verifier with path = %T, want ExternalUCI", server.opts.Verifier)
	}
	if err := server.setOption("setoption name VerifierEnabled value false"); err != nil {
		t.Fatalf("disable verifier: %v", err)
	}
	if err := server.setOption("setoption name VerifierMoveTime value 250"); err != nil {
		t.Fatalf("set verifier movetime: %v", err)
	}
	if _, ok := server.opts.Verifier.(verifier.ExternalUCI); ok {
		t.Fatal("verifier timing update re-enabled disabled external verifier")
	}
	static, ok := server.opts.Verifier.(verifier.StaticVerifier)
	if !ok || static.Enabled {
		t.Fatalf("disabled verifier = %#v, want disabled static verifier", server.opts.Verifier)
	}
}

func TestUCIReadyAndStopDuringActiveSearch(t *testing.T) {
	input := strings.Join([]string{
		"uci",
		"position startpos",
		"go movetime 1000",
		"isready",
		"stop",
		"quit",
		"",
	}, "\n")
	var out bytes.Buffer
	settings := storage.DefaultSettings()
	settings.Logging.OutputDir = t.TempDir()
	server := NewServer(strings.NewReader(input), &out, &bytes.Buffer{}, settings)
	server.opts.Provider = providers.MockProvider{Behavior: "slow"}
	server.engine.SetOptions(server.opts)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	text := out.String()
	readyIndex := strings.Index(text, "readyok")
	bestIndex := strings.Index(text, "bestmove ")
	if readyIndex < 0 || bestIndex < 0 {
		t.Fatalf("missing readyok or bestmove:\n%s", text)
	}
	if readyIndex > bestIndex {
		t.Fatalf("readyok should be emitted while search is active before bestmove:\n%s", text)
	}
	if strings.Contains(text, "bestmove 0000") {
		t.Fatalf("stop returned null move despite legal moves:\n%s", text)
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if !validUCILine(line) {
			t.Fatalf("non-UCI stdout line: %q\n%s", line, text)
		}
	}
}

func TestUCIStopAfterCancelledSearchReturnsLegalFallback(t *testing.T) {
	input := strings.Join([]string{
		"uci",
		"position startpos",
		"go movetime 1000",
		"stop",
		"quit",
		"",
	}, "\n")
	var out bytes.Buffer
	settings := storage.DefaultSettings()
	settings.Logging.OutputDir = t.TempDir()
	server := NewServer(strings.NewReader(input), &out, &bytes.Buffer{}, settings)
	server.opts.Provider = cancelThenRespondProvider{}
	server.engine.SetOptions(server.opts)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "bestmove ") {
		t.Fatalf("missing bestmove:\n%s", text)
	}
	if strings.Contains(text, "bestmove 0000") {
		t.Fatalf("stop returned null move despite legal moves:\n%s", text)
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if !validUCILine(line) {
			t.Fatalf("non-UCI stdout line: %q\n%s", line, text)
		}
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

type cancelThenRespondProvider struct{}

func (cancelThenRespondProvider) Name() string {
	return "cancel-then-respond"
}

func (cancelThenRespondProvider) Capabilities() providers.Capabilities {
	return providers.MockProvider{}.Capabilities()
}

func (cancelThenRespondProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (cancelThenRespondProvider) CompleteJSON(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	<-ctx.Done()
	time.Sleep(10 * time.Millisecond)
	return providers.MockProvider{}.CompleteJSON(context.Background(), req)
}
