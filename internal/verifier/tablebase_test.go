package verifier

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestTablebaseVerifierRejectsNonBestCandidates(t *testing.T) {
	candidates := tablebaseCandidates("h1h2", "h1g1")
	result, err := (TablebaseVerifier{
		Base:    LegalOnlyVerifier{},
		Probe:   stubTablebaseProbe{result: &TablebaseResult{Available: true, BestMoves: []string{"h1h2"}, WDL: "win", DTZ: 1, Category: "win"}},
		Enabled: true,
	}).VerifyCandidates(context.Background(), Request{
		FEN:        tablebaseTestFEN,
		Candidates: candidates,
		Mode:       strategy.ModeHybrid,
	})
	if err != nil {
		t.Fatalf("verify candidates: %v", err)
	}
	if !result.Used {
		t.Fatal("expected tablebase result to mark verifier as used")
	}
	byMove := map[string]CandidateResult{}
	for _, item := range result.Candidates {
		byMove[item.UCI] = item
	}
	if byMove["h1h2"].Status != "accepted" {
		t.Fatalf("best move status = %+v", byMove["h1h2"])
	}
	if byMove["h1g1"].Status != "rejected" || byMove["h1g1"].Details["tablebase_wdl"] != "win" {
		t.Fatalf("non-best move status = %+v", byMove["h1g1"])
	}
}

func TestTablebaseVerifierFallsBackWhenUnavailable(t *testing.T) {
	candidates := tablebaseCandidates("h1h2")
	result, err := (TablebaseVerifier{
		Base:    LegalOnlyVerifier{},
		Probe:   stubTablebaseProbe{result: &TablebaseResult{Available: false}},
		Enabled: true,
	}).VerifyCandidates(context.Background(), Request{
		FEN:        tablebaseTestFEN,
		Candidates: candidates,
		Mode:       strategy.ModePure,
	})
	if err != nil {
		t.Fatalf("verify candidates: %v", err)
	}
	if result.Used {
		t.Fatal("unavailable tablebase should not mark result as used")
	}
	if result.Candidates[0].Details["tablebase_available"] != "false" {
		t.Fatalf("missing unavailable detail: %+v", result.Candidates[0])
	}
}

func TestTablebaseVerifierRecordsProbeFailure(t *testing.T) {
	result, err := (TablebaseVerifier{
		Base:    LegalOnlyVerifier{},
		Probe:   stubTablebaseProbe{err: context.DeadlineExceeded},
		Enabled: true,
	}).VerifyCandidates(context.Background(), Request{
		FEN:        tablebaseTestFEN,
		Candidates: tablebaseCandidates("h1h2"),
		Mode:       strategy.ModeHybrid,
	})
	if err != nil {
		t.Fatalf("verify candidates: %v", err)
	}
	if result.Error == "" || result.Used {
		t.Fatalf("probe failure should be recorded without marking tablebase used: %+v", result)
	}
}

func TestExternalTablebaseProbe(t *testing.T) {
	probePath := buildFakeTablebaseProbe(t)
	result, err := (ExternalTablebase{Path: probePath, TimeoutMS: 2000}).Probe(context.Background(), Request{
		FEN:        tablebaseTestFEN,
		Candidates: tablebaseCandidates("h1h2"),
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !result.Available || len(result.BestMoves) != 1 || result.BestMoves[0] != "h1h2" || result.WDL != "win" {
		t.Fatalf("unexpected tablebase result: %+v", result)
	}
}

func TestNativeSyzygyProbeHandlesKingsOnlyDraw(t *testing.T) {
	result, err := (NativeSyzygyProbe{Path: t.TempDir()}).Probe(context.Background(), Request{
		FEN:        tablebaseTestFEN,
		Candidates: tablebaseCandidates("a1a2", "a1b1"),
	})
	if err != nil {
		t.Fatalf("native probe: %v", err)
	}
	if !result.Available || result.WDL != "draw" || result.Category != "native_kings_only" || len(result.BestMoves) != 2 {
		t.Fatalf("unexpected native result: %+v", result)
	}
}

func buildFakeTablebaseProbe(t *testing.T) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable for fake tablebase probe build")
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	probePath := filepath.Join(dir, "fake-tablebase")
	source := `package main

import (
	"encoding/json"
	"os"
)

type request struct {
	FEN        string
	Candidates []string
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil || req.FEN == "" || len(req.Candidates) == 0 {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"available": false})
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"available":  true,
		"best_moves": []string{"h1h2"},
		"wdl":        "win",
		"dtz":        1,
		"category":   "win",
	})
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write fake tablebase source: %v", err)
	}
	cmd := exec.Command(goTool, "build", "-o", probePath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake tablebase: %v\n%s", err, out)
	}
	warmTestExecutable(t, probePath)
	return probePath
}

type stubTablebaseProbe struct {
	result *TablebaseResult
	err    error
}

func (p stubTablebaseProbe) Name() string {
	return "stub_tablebase"
}

func (p stubTablebaseProbe) Probe(ctx context.Context, req Request) (*TablebaseResult, error) {
	return p.result, p.err
}

const tablebaseTestFEN = "8/8/8/8/8/8/8/K6k w - - 0 1"

func tablebaseCandidates(moves ...string) []strategy.CandidateMove {
	out := []strategy.CandidateMove{}
	for _, move := range moves {
		out = append(out, strategy.CandidateMove{UCI: move})
	}
	return out
}
