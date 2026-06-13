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
	for _, want := range []string{
		"id name Noema64",
		"uciok",
		"readyok",
		"bestmove ",
		"option name MoveOverhead",
		"option name MaxProviderMillis",
		"option name MaxVerifierMillis",
		"option name TablebaseEnabled",
		"option name TablebasePath",
		"option name TablebaseTimeoutMS",
		"option name LogPath",
		"info string noema64 provider=",
		"info string noema64 selected=",
	} {
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

func TestUCITwoHundredScriptedSessions(t *testing.T) {
	settings := storage.DefaultSettings()
	settings.Engine.TraceEnabled = false
	settings.Logging.OutputDir = t.TempDir()
	script := strings.Join([]string{
		"uci",
		"isready",
		"ucinewgame",
		"position startpos moves e2e4 e7e5 g1f3 b8c6",
		"go movetime 50",
		"quit",
		"",
	}, "\n")
	for i := 0; i < 200; i++ {
		var out bytes.Buffer
		server := NewServer(strings.NewReader(script), &out, &bytes.Buffer{}, settings)
		if err := server.Run(context.Background()); err != nil {
			t.Fatalf("session %d run: %v", i, err)
		}
		text := out.String()
		if !strings.Contains(text, "uciok") || !strings.Contains(text, "readyok") || !strings.Contains(text, "bestmove ") {
			t.Fatalf("session %d missing required UCI output:\n%s", i, text)
		}
		if strings.Contains(text, "bestmove 0000") {
			t.Fatalf("session %d returned null move despite legal moves:\n%s", i, text)
		}
		for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
			if !validUCILine(line) {
				t.Fatalf("session %d non-UCI stdout line: %q\n%s", i, line, text)
			}
		}
	}
}

func TestUCITracePathOptions(t *testing.T) {
	for _, optionName := range []string{"TraceFile", "LogPath"} {
		t.Run(optionName, func(t *testing.T) {
			tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
			input := strings.Join([]string{
				"uci",
				"setoption name " + optionName + " value " + tracePath,
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
		})
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
	if err := server.setOption("setoption name LLMRetries value 99"); err != nil {
		t.Fatalf("set retries: %v", err)
	}
	if server.opts.ProviderRetries != 5 || server.providerRetries != 5 {
		t.Fatalf("provider retries = opts:%d server:%d, want 5", server.opts.ProviderRetries, server.providerRetries)
	}
	if err := server.setOption("setoption name MoveOverhead value 99999"); err != nil {
		t.Fatalf("set move overhead: %v", err)
	}
	if server.moveOverheadMS != 5000 {
		t.Fatalf("move overhead = %d, want 5000", server.moveOverheadMS)
	}
	if err := server.setOption("setoption name MoveOverhead value -25"); err != nil {
		t.Fatalf("set move overhead low: %v", err)
	}
	if server.moveOverheadMS != 0 {
		t.Fatalf("move overhead = %d, want 0", server.moveOverheadMS)
	}
	if err := server.setOption("setoption name MaxProviderMillis value 999999"); err != nil {
		t.Fatalf("set max provider millis: %v", err)
	}
	if server.maxProviderMS != 120000 || server.opts.MoveTimeout != 120*time.Second {
		t.Fatalf("max provider millis = %d timeout=%s, want 120000/120s", server.maxProviderMS, server.opts.MoveTimeout)
	}
	if err := server.setOption("setoption name MaxProviderMillis value 1"); err != nil {
		t.Fatalf("set max provider millis low: %v", err)
	}
	if server.maxProviderMS != 100 || server.opts.MoveTimeout != 100*time.Millisecond {
		t.Fatalf("max provider millis = %d timeout=%s, want 100/100ms", server.maxProviderMS, server.opts.MoveTimeout)
	}
	if err := server.setOption("setoption name VerifierPath value /usr/bin/stockfish"); err != nil {
		t.Fatalf("set verifier path: %v", err)
	}
	if err := server.setOption("setoption name VerifierEnabled value true"); err != nil {
		t.Fatalf("enable verifier: %v", err)
	}
	if err := server.setOption("setoption name VerifierMoveTime value 99999"); err != nil {
		t.Fatalf("set verifier movetime: %v", err)
	}
	external, ok := server.opts.Verifier.(verifier.ExternalUCI)
	if !ok {
		t.Fatalf("verifier = %T, want ExternalUCI", server.opts.Verifier)
	}
	if external.MoveTimeMS != 5000 {
		t.Fatalf("verifier movetime = %d, want 5000", external.MoveTimeMS)
	}
	if err := server.setOption("setoption name VerifierMoveTime value 1"); err != nil {
		t.Fatalf("set verifier movetime low: %v", err)
	}
	external = server.opts.Verifier.(verifier.ExternalUCI)
	if external.MoveTimeMS != 10 {
		t.Fatalf("verifier movetime = %d, want 10", external.MoveTimeMS)
	}
	if err := server.setOption("setoption name MaxVerifierMillis value 99999"); err != nil {
		t.Fatalf("set max verifier millis: %v", err)
	}
	external = server.opts.Verifier.(verifier.ExternalUCI)
	if external.MoveTimeMS != 60000 {
		t.Fatalf("max verifier millis = %d, want 60000", external.MoveTimeMS)
	}
	if err := server.setOption("setoption name VerifierMaxCentipawnLoss value 99999"); err != nil {
		t.Fatalf("set verifier max loss: %v", err)
	}
	external = server.opts.Verifier.(verifier.ExternalUCI)
	if external.MaxCentipawnLoss != 2000 {
		t.Fatalf("verifier max loss = %d, want 2000", external.MaxCentipawnLoss)
	}
	if err := server.setOption("setoption name VerifierMaxCentipawnLoss value 0"); err != nil {
		t.Fatalf("set verifier max loss zero: %v", err)
	}
	external = server.opts.Verifier.(verifier.ExternalUCI)
	if external.MaxCentipawnLoss != 0 {
		t.Fatalf("verifier max loss = %d, want 0", external.MaxCentipawnLoss)
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

func TestUCITablebaseOptionsWrapVerifier(t *testing.T) {
	server := NewServer(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, storage.DefaultSettings())
	if err := server.setOption("setoption name TablebasePath value /usr/bin/noema64-tablebase"); err != nil {
		t.Fatalf("set tablebase path: %v", err)
	}
	if _, ok := server.opts.Verifier.(verifier.TablebaseVerifier); ok {
		t.Fatal("tablebase path enabled wrapper without TablebaseEnabled=true")
	}
	if err := server.setOption("setoption name TablebaseEnabled value true"); err != nil {
		t.Fatalf("enable tablebase: %v", err)
	}
	if err := server.setOption("setoption name TablebaseTimeoutMS value 99999"); err != nil {
		t.Fatalf("set tablebase timeout: %v", err)
	}
	tb, ok := server.opts.Verifier.(verifier.TablebaseVerifier)
	if !ok {
		t.Fatalf("verifier = %T, want TablebaseVerifier", server.opts.Verifier)
	}
	probe, ok := tb.Probe.(verifier.ExternalTablebase)
	if !ok || probe.TimeoutMS != 10000 || probe.Path != "/usr/bin/noema64-tablebase" {
		t.Fatalf("tablebase probe = %#v", tb.Probe)
	}
	if err := server.setOption("setoption name VerifierPath value /usr/bin/stockfish"); err != nil {
		t.Fatalf("set verifier path: %v", err)
	}
	if err := server.setOption("setoption name VerifierEnabled value true"); err != nil {
		t.Fatalf("enable verifier: %v", err)
	}
	tb = server.opts.Verifier.(verifier.TablebaseVerifier)
	if _, ok := tb.Base.(verifier.ExternalUCI); !ok {
		t.Fatalf("tablebase base = %T, want ExternalUCI", tb.Base)
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

func TestUCIPonderhitDuringActiveSearch(t *testing.T) {
	input := strings.Join([]string{
		"uci",
		"position startpos",
		"go ponder movetime 1000",
		"ponderhit",
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
	if !strings.Contains(text, "info string ponderhit accepted") {
		t.Fatalf("missing ponderhit acknowledgement:\n%s", text)
	}
	if !strings.Contains(text, "bestmove ") {
		t.Fatalf("missing bestmove after ponderhit:\n%s", text)
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if !validUCILine(line) {
			t.Fatalf("non-UCI stdout line: %q\n%s", line, text)
		}
	}
}

func TestUCIPonderhitWithoutActiveSearch(t *testing.T) {
	var out bytes.Buffer
	server := NewServer(strings.NewReader(""), &out, &bytes.Buffer{}, storage.DefaultSettings())
	if err := server.handle(context.Background(), "ponderhit"); err != nil {
		t.Fatalf("ponderhit: %v", err)
	}
	if !strings.Contains(out.String(), "info string ponderhit ignored: no active search") {
		t.Fatalf("unexpected ponderhit output:\n%s", out.String())
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
