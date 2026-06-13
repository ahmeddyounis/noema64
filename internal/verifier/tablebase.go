package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/security"
)

type TablebaseProbe interface {
	Name() string
	Probe(ctx context.Context, req Request) (*TablebaseResult, error)
}

type TablebaseRequest struct {
	FEN        string   `json:"fen"`
	Candidates []string `json:"candidates"`
}

type TablebaseResult struct {
	Available bool     `json:"available"`
	BestMoves []string `json:"best_moves"`
	WDL       string   `json:"wdl,omitempty"`
	DTZ       int      `json:"dtz,omitempty"`
	Category  string   `json:"category,omitempty"`
}

type ExternalTablebase struct {
	Path      string
	TimeoutMS int
}

type NativeSyzygyProbe struct {
	Path string
}

func (p NativeSyzygyProbe) Name() string {
	if p.Path == "" {
		return "native_syzygy"
	}
	return "native_syzygy:" + p.Path
}

func (p NativeSyzygyProbe) Probe(ctx context.Context, req Request) (*TablebaseResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	files := discoverSyzygyFiles(p.Path)
	pieces := materialPieces(req.FEN)
	if len(pieces) == 0 || len(pieces) > 7 {
		return &TablebaseResult{Available: false, Category: "unsupported_piece_count"}, nil
	}
	if onlyKings(pieces) {
		return &TablebaseResult{Available: true, BestMoves: candidateUCIs(req), WDL: "draw", Category: "native_kings_only"}, nil
	}
	if isBasicMateMaterial(pieces) {
		best := bestBasicMateMoves(req)
		if len(best) > 0 {
			return &TablebaseResult{Available: true, BestMoves: best, WDL: "win", Category: "native_basic_mate"}, nil
		}
	}
	if len(files) == 0 {
		return &TablebaseResult{Available: false, Category: "no_syzygy_files"}, nil
	}
	return &TablebaseResult{Available: false, Category: "native_decoder_unavailable"}, nil
}

func (p ExternalTablebase) Name() string {
	if p.Path == "" {
		return "external_tablebase"
	}
	return "external_tablebase:" + p.Path
}

func (p ExternalTablebase) Probe(ctx context.Context, req Request) (*TablebaseResult, error) {
	if p.Path == "" {
		return nil, fmt.Errorf("tablebase probe path is empty")
	}
	commandPath, err := security.ValidateExternalCommand(p.Path, security.DefaultExternalCommandPolicy())
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(p.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	payload := TablebaseRequest{FEN: req.FEN}
	for _, candidate := range req.Candidates {
		payload.Candidates = append(payload.Candidates, candidate.UCI)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	stdinFile, err := os.CreateTemp("", "noema64-tablebase-stdin-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(stdinFile.Name())
	defer stdinFile.Close()
	if _, err := stdinFile.Write(b); err != nil {
		return nil, err
	}
	if _, err := stdinFile.Seek(0, 0); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(probeCtx, commandPath)
	cmd.Stdin = stdinFile
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		if probeCtx.Err() != nil {
			return nil, probeCtx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	var result TablebaseResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type TablebaseVerifier struct {
	Base    Verifier
	Probe   TablebaseProbe
	Enabled bool
}

func (v TablebaseVerifier) Name() string {
	baseName := "none"
	if v.Base != nil {
		baseName = v.Base.Name()
	}
	if v.Probe == nil {
		return baseName + "+tablebase"
	}
	return baseName + "+" + v.Probe.Name()
}

func (v TablebaseVerifier) VerifyCandidates(ctx context.Context, req Request) (*Result, error) {
	base := v.Base
	if base == nil {
		base = StaticVerifier{Enabled: true}
	}
	result, err := base.VerifyCandidates(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &Result{}
	}
	if !v.Enabled || v.Probe == nil {
		return result, nil
	}
	result.Name = v.Name()
	tb, err := v.Probe.Probe(ctx, req)
	if err != nil {
		result.Error = "tablebase probe failed: " + err.Error()
		return result, nil
	}
	if tb == nil || !tb.Available || len(tb.BestMoves) == 0 {
		addTablebaseDetails(result, map[string]string{"tablebase_available": "false"})
		return result, nil
	}
	result.Enabled = true
	result.Used = true
	result.Name = v.Name()
	byMove := candidateResultsByMove(result.Candidates)
	result.Candidates = result.Candidates[:0]
	for _, candidate := range req.Candidates {
		item := byMove[candidate.UCI]
		item.UCI = candidate.UCI
		item.Details = cloneDetails(item.Details)
		if item.Details == nil {
			item.Details = map[string]string{}
		}
		item.Details["tablebase_available"] = "true"
		item.Details["tablebase_wdl"] = tb.WDL
		item.Details["tablebase_category"] = tb.Category
		item.Details["tablebase_best_moves"] = joinMoves(tb.BestMoves)
		item.Details["tablebase_dtz"] = fmt.Sprintf("%d", tb.DTZ)
		if slices.Contains(tb.BestMoves, candidate.UCI) {
			item.Status = "accepted"
			item.Reason = "Tablebase marks this candidate as an exact best move."
			item.MateRisk = false
			item.Score = strategyScore(item.Status, item.Reason, 0, false)
		} else {
			item.Status = "rejected"
			item.Reason = "Tablebase rejects this candidate because an exact best move is available."
			item.MateRisk = false
			item.Score = strategyScore(item.Status, item.Reason, 2000, false)
		}
		result.Candidates = append(result.Candidates, item)
	}
	return result, nil
}

func candidateResultsByMove(items []CandidateResult) map[string]CandidateResult {
	out := map[string]CandidateResult{}
	for _, item := range items {
		out[item.UCI] = item
	}
	return out
}

func addTablebaseDetails(result *Result, details map[string]string) {
	for i := range result.Candidates {
		if result.Candidates[i].Details == nil {
			result.Candidates[i].Details = map[string]string{}
		}
		for key, value := range details {
			result.Candidates[i].Details[key] = value
		}
	}
}

func cloneDetails(details map[string]string) map[string]string {
	if details == nil {
		return nil
	}
	out := make(map[string]string, len(details))
	for key, value := range details {
		out[key] = value
	}
	return out
}

func joinMoves(moves []string) string {
	return strings.Join(moves, ",")
}

func discoverSyzygyFiles(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".rtbw" || ext == ".rtbz" {
			return []string{path}
		}
		return nil
	}
	files := []string{}
	for _, pattern := range []string{"*.rtbw", "*.rtbz"} {
		matches, _ := filepath.Glob(filepath.Join(path, pattern))
		files = append(files, matches...)
	}
	sortStrings(files)
	return files
}

func materialPieces(fen string) []rune {
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return nil
	}
	out := []rune{}
	for _, ch := range fields[0] {
		if ch == '/' || (ch >= '1' && ch <= '8') {
			continue
		}
		out = append(out, ch)
	}
	return out
}

func onlyKings(pieces []rune) bool {
	if len(pieces) != 2 {
		return false
	}
	hasWhiteKing := false
	hasBlackKing := false
	for _, piece := range pieces {
		switch piece {
		case 'K':
			hasWhiteKing = true
		case 'k':
			hasBlackKing = true
		default:
			return false
		}
	}
	return hasWhiteKing && hasBlackKing
}

func isBasicMateMaterial(pieces []rune) bool {
	if len(pieces) != 3 {
		return false
	}
	hasWhiteKing := false
	hasBlackKing := false
	hasMajor := false
	for _, piece := range pieces {
		switch piece {
		case 'K':
			hasWhiteKing = true
		case 'k':
			hasBlackKing = true
		case 'Q', 'q', 'R', 'r':
			hasMajor = true
		default:
			return false
		}
	}
	return hasWhiteKing && hasBlackKing && hasMajor
}

func bestBasicMateMoves(req Request) []string {
	if req.Game == nil {
		return candidateUCIs(req)
	}
	checkmates := []string{}
	checks := []string{}
	for _, candidate := range req.Candidates {
		clone := req.Game.Clone()
		if _, err := clone.ApplyUCI(candidate.UCI); err != nil {
			continue
		}
		if clone.Outcome().Status == "checkmate" {
			checkmates = append(checkmates, candidate.UCI)
			continue
		}
		for _, legal := range req.Game.LegalMoves() {
			if legal.UCI == candidate.UCI && legal.Check {
				checks = append(checks, candidate.UCI)
				break
			}
		}
	}
	if len(checkmates) > 0 {
		return checkmates
	}
	if len(checks) > 0 {
		return checks
	}
	return candidateUCIs(req)
}

func candidateUCIs(req Request) []string {
	out := make([]string, 0, len(req.Candidates))
	for _, candidate := range req.Candidates {
		out = append(out, candidate.UCI)
	}
	return out
}

func sortStrings(items []string) {
	slices.Sort(items)
}
