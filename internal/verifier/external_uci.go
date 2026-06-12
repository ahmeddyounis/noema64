package verifier

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

type ExternalUCI struct {
	Path             string
	MoveTimeMS       int
	MaxCentipawnLoss int
}

const externalUCICleanupTimeout = 500 * time.Millisecond

func (v ExternalUCI) Name() string {
	if v.Path == "" {
		return "external_uci"
	}
	return "external_uci:" + v.Path
}

func (v ExternalUCI) HealthCheck(ctx context.Context) error {
	if v.Path == "" {
		return fmt.Errorf("external verifier path is empty")
	}
	healthCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(healthCtx, v.Path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer stopUCIProcess(cmd, stdin)
	reader := bufio.NewReader(stdout)
	if _, err := io.WriteString(stdin, "uci\n"); err != nil {
		return err
	}
	if err := waitForToken(healthCtx, reader, "uciok"); err != nil {
		return fmt.Errorf("external verifier did not complete UCI handshake: %w", err)
	}
	if _, err := io.WriteString(stdin, "isready\n"); err != nil {
		return err
	}
	if err := waitForToken(healthCtx, reader, "readyok"); err != nil {
		return fmt.Errorf("external verifier did not become ready: %w", err)
	}
	return nil
}

func (v ExternalUCI) VerifyCandidates(ctx context.Context, req Request) (*Result, error) {
	if v.Path == "" {
		return nil, fmt.Errorf("external verifier path is empty")
	}
	moveTime := v.MoveTimeMS
	if moveTime <= 0 {
		moveTime = 100
	}
	maxLoss := v.MaxCentipawnLoss
	if maxLoss < 0 {
		maxLoss = 180
	}
	cmd := exec.CommandContext(ctx, v.Path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(stdout)
	defer stopUCIProcess(cmd, stdin)
	if _, err := io.WriteString(stdin, "uci\n"); err != nil {
		return nil, err
	}
	if err := waitForToken(ctx, reader, "uciok"); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(stdin, "isready\n"); err != nil {
		return nil, err
	}
	if err := waitForToken(ctx, reader, "readyok"); err != nil {
		return nil, err
	}

	staticResult, _ := (StaticVerifier{Enabled: true}).VerifyCandidates(ctx, req)
	scores := map[string]int{}
	best := -1 << 30
	result := &Result{Enabled: true, Used: true, Name: v.Name()}
	for _, candidate := range req.Candidates {
		score, err := v.analyzeCandidate(ctx, stdin, reader, req.FEN, candidate.UCI, moveTime)
		if err != nil {
			return nil, err
		}
		scores[candidate.UCI] = score
		if score > best {
			best = score
		}
	}
	staticByMove := map[string]CandidateResult{}
	if staticResult != nil {
		for _, item := range staticResult.Candidates {
			staticByMove[item.UCI] = item
		}
	}
	for _, candidate := range req.Candidates {
		score := scores[candidate.UCI]
		loss := best - score
		status := "accepted"
		reason := fmt.Sprintf("External verifier score %d cp; loss versus best candidate %d cp.", score, loss)
		mateRisk := false
		if static, ok := staticByMove[candidate.UCI]; ok && static.Status == "rejected" {
			status = "rejected"
			reason = static.Reason
			mateRisk = static.MateRisk
		} else if score <= -90000 {
			status = "rejected"
			reason = "External verifier reports a forced mate against the side to move."
			mateRisk = true
		} else if loss > maxLoss {
			status = "rejected"
			reason = fmt.Sprintf("External verifier rejects candidate: %d cp worse than best candidate.", loss)
		} else if loss > maxLoss/2 {
			status = "warning"
			reason = fmt.Sprintf("External verifier warns candidate is %d cp worse than best candidate.", loss)
		}
		result.Candidates = append(result.Candidates, CandidateResult{
			UCI:      candidate.UCI,
			Status:   status,
			Reason:   reason,
			MateRisk: mateRisk,
			Score:    strategyScore(status, reason, loss, mateRisk),
		})
	}
	return result, nil
}

func (v ExternalUCI) analyzeCandidate(ctx context.Context, stdin io.Writer, reader *bufio.Reader, fen, moveUCI string, moveTime int) (int, error) {
	if _, err := fmt.Fprintf(stdin, "position fen %s\n", fen); err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintf(stdin, "go movetime %d searchmoves %s\n", moveTime, moveUCI); err != nil {
		return 0, err
	}
	score := 0
	foundScore := false
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimSpace(line)
		if parsed, ok := parseUCIScore(line); ok {
			score = parsed
			foundScore = true
		}
		if strings.HasPrefix(line, "bestmove ") {
			if !foundScore {
				return 0, nil
			}
			return score, nil
		}
	}
}

func stopUCIProcess(cmd *exec.Cmd, stdin io.Writer) {
	if stdin != nil {
		_, _ = io.WriteString(stdin, "quit\n")
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(externalUCICleanupTimeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
	}
}

func waitForToken(ctx context.Context, reader *bufio.Reader, token string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(line) == token {
			return nil
		}
	}
}

func parseUCIScore(line string) (int, bool) {
	fields := strings.Fields(line)
	for i := 0; i+2 < len(fields); i++ {
		if fields[i] != "score" {
			continue
		}
		value, err := strconv.Atoi(fields[i+2])
		if err != nil {
			return 0, false
		}
		switch fields[i+1] {
		case "cp":
			return value, true
		case "mate":
			if value < 0 {
				return -100000 - value, true
			}
			return 100000 - value, true
		}
	}
	return 0, false
}

func strategyScore(status, reason string, loss int, mateRisk bool) strategy.VerifierScore {
	return strategy.VerifierScore{
		Status:        status,
		CentipawnLoss: loss,
		MateRisk:      mateRisk,
		Reason:        reason,
	}
}
