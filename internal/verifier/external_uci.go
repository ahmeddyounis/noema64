package verifier

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type ExternalUCI struct {
	Path       string
	MoveTimeMS int
}

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
	cmd := exec.CommandContext(ctx, v.Path)
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
	defer func() {
		_, _ = io.WriteString(stdin, "quit\n")
		_ = cmd.Wait()
	}()
	reader := bufio.NewReader(stdout)
	if _, err := io.WriteString(stdin, "uci\nisready\n"); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(line) == "readyok" {
			return nil
		}
	}
	return fmt.Errorf("external verifier did not become ready")
}

func (v ExternalUCI) VerifyCandidates(ctx context.Context, req Request) (*Result, error) {
	if v.Path == "" {
		return nil, fmt.Errorf("external verifier path is empty")
	}
	if err := v.HealthCheck(ctx); err != nil {
		return nil, err
	}
	static := StaticVerifier{Enabled: true}
	result, err := static.VerifyCandidates(ctx, req)
	if result != nil {
		result.Name = v.Name()
	}
	return result, err
}
