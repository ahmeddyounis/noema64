package uci

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/finetune"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/security"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"github.com/ahmedyounis/noema64/internal/verifier"
)

const EngineName = "Noema64 0.1.0"

type Server struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer

	mu               sync.Mutex
	outMu            sync.Mutex
	engine           *engine.Engine
	traceStore       *storage.TraceStore
	opts             engine.Options
	providerKind     string
	endpoint         string
	apiKey           string
	providerRetries  int
	moveOverheadMS   int
	maxProviderMS    int
	verifierEnabled  bool
	verifierPath     string
	verifierMoveTime int
	verifierMaxLoss  int
	tablebaseEnabled bool
	tablebasePath    string
	tablebaseTimeout int
	traceEnabled     bool
	searchCancel     context.CancelFunc
	searchDone       chan struct{}
}

func NewServer(in io.Reader, out io.Writer, errOut io.Writer, settings storage.Settings) *Server {
	opts := engineOptions(settings)
	traceDir := settings.Logging.OutputDir
	if traceDir == "" {
		traceDir = "logs"
	}
	maxProviderMS := settings.LLM.TimeoutMS
	if maxProviderMS <= 0 {
		maxProviderMS = 12000
	}
	return &Server{
		in:               in,
		out:              out,
		errOut:           errOut,
		opts:             opts,
		engine:           engine.New(opts),
		traceStore:       storage.NewTraceStore(traceDir),
		providerKind:     settings.LLM.Provider,
		endpoint:         settings.LLM.Endpoint,
		apiKey:           settings.LLM.APIKey,
		providerRetries:  settings.LLM.Retries,
		moveOverheadMS:   100,
		maxProviderMS:    clampInt(maxProviderMS, 100, 120000),
		verifierEnabled:  settings.Verifier.Enabled,
		verifierPath:     settings.Verifier.Path,
		verifierMoveTime: settings.Verifier.MoveTimeMS,
		verifierMaxLoss:  settings.Verifier.MaxCentipawnLoss,
		tablebaseEnabled: settings.Verifier.TablebaseEnabled,
		tablebasePath:    settings.Verifier.TablebasePath,
		tablebaseTimeout: settings.Verifier.TablebaseTimeoutMS,
		traceEnabled:     settings.Engine.TraceEnabled,
	}
}

func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := s.handle(ctx, line); err != nil {
			s.info("warning: " + err.Error())
		}
		if line == "quit" {
			return nil
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "uci":
		s.write("id name " + EngineName)
		s.write("id author Noema64 contributors")
		s.write("option name Mode type combo default blunderguard var pure var blunderguard var hybrid var coach")
		s.write("option name Personality type combo default balanced var balanced var aggressive var positional var beginner_coach")
		s.write("option name LLMProvider type string default mock")
		s.write("option name LLMModel type string default mock-balanced")
		s.write("option name LLMEndpoint type string default")
		s.write("option name Temperature type spin default 20 min 0 max 200")
		s.write("option name MaxCandidates type spin default 5 min 1 max 10")
		s.write("option name LLMRetries type spin default 1 min 0 max 5")
		s.write("option name MoveOverhead type spin default 100 min 0 max 5000")
		s.write("option name MaxProviderMillis type spin default 12000 min 100 max 120000")
		s.write("option name VerifierEnabled type check default false")
		s.write("option name VerifierPath type string default")
		s.write("option name VerifierMoveTime type spin default 100 min 10 max 5000")
		s.write("option name MaxVerifierMillis type spin default 1000 min 0 max 60000")
		s.write("option name VerifierMaxCentipawnLoss type spin default 180 min 0 max 2000")
		s.write("option name TablebaseEnabled type check default false")
		s.write("option name TablebasePath type string default")
		s.write("option name TablebaseTimeoutMS type spin default 1000 min 50 max 10000")
		s.write("option name TraceEnabled type check default true")
		s.write("option name TraceFile type string default")
		s.write("option name LogPath type string default")
		s.write("uciok")
	case "isready":
		s.write("readyok")
	case "ucinewgame":
		_, err := s.engine.NewGame(ctx, engine.NewGameOptions{Side: "auto"})
		return err
	case "setoption":
		return s.setOption(line)
	case "position":
		return s.position(ctx, fields[1:])
	case "go":
		return s.goCommand(ctx, fields[1:])
	case "ponderhit":
		s.ponderhit()
	case "stop":
		s.stop()
	case "quit":
		s.stop()
	default:
		s.info("ignored unsupported command: " + fields[0])
	}
	return nil
}

func (s *Server) setOption(line string) error {
	name, value := parseSetOption(line)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch strings.ToLower(name) {
	case "mode":
		s.opts.Mode = strategy.EngineMode(value)
	case "personality", "strategypersonality":
		s.opts.Personality = strategy.Personality(value)
	case "llmprovider", "providerprofile":
		s.providerKind = value
		s.refreshProviderLocked()
	case "llmmodel":
		s.opts.Model = value
		s.refreshProviderLocked()
	case "llmendpoint":
		s.endpoint = value
		s.refreshProviderLocked()
	case "llmretries", "retries":
		n, err := strconv.Atoi(value)
		if err == nil && n >= 0 {
			s.providerRetries = clampInt(n, 0, 5)
			s.opts.ProviderRetries = s.providerRetries
			s.refreshProviderLocked()
		}
	case "temperature":
		n, err := strconv.Atoi(value)
		if err == nil {
			s.opts.Temperature = float64(clampInt(n, 0, 200)) / 100
		}
	case "maxcandidates":
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			s.opts.MaxCandidates = clampInt(n, 1, 10)
		}
	case "moveoverhead":
		n, err := strconv.Atoi(value)
		if err == nil {
			s.moveOverheadMS = clampInt(n, 0, 5000)
		}
	case "maxprovidermillis":
		n, err := strconv.Atoi(value)
		if err == nil {
			s.maxProviderMS = clampInt(n, 100, 120000)
			s.opts.MoveTimeout = time.Duration(s.maxProviderMS) * time.Millisecond
		}
	case "verifierenabled":
		s.verifierEnabled = strings.EqualFold(value, "true")
		s.refreshVerifierLocked(s.verifierEnabled)
	case "verifierpath":
		s.verifierPath = value
		s.refreshVerifierLocked(s.verifierEnabled)
	case "verifiermovetime":
		n, err := strconv.Atoi(value)
		if err == nil {
			s.verifierMoveTime = clampInt(n, 10, 5000)
			s.refreshVerifierLocked(s.verifierEnabled)
		}
	case "maxverifiermillis":
		n, err := strconv.Atoi(value)
		if err == nil {
			s.verifierMoveTime = clampInt(n, 0, 60000)
			s.refreshVerifierLocked(s.verifierEnabled)
		}
	case "verifiermaxcentipawnloss":
		n, err := strconv.Atoi(value)
		if err == nil && n >= 0 {
			s.verifierMaxLoss = clampInt(n, 0, 2000)
			s.refreshVerifierLocked(s.verifierEnabled)
		}
	case "tablebaseenabled":
		s.tablebaseEnabled = strings.EqualFold(value, "true")
		s.refreshVerifierLocked(s.verifierEnabled)
	case "tablebasepath":
		s.tablebasePath = value
		s.refreshVerifierLocked(s.verifierEnabled)
	case "tablebasetimeout", "tablebasetimeoutms":
		n, err := strconv.Atoi(value)
		if err == nil {
			s.tablebaseTimeout = clampInt(n, 50, 10000)
			s.refreshVerifierLocked(s.verifierEnabled)
		}
	case "tracefile", "logpath":
		if value != "" {
			s.traceStore = storage.NewTraceFileStore(value)
		}
	case "traceenabled":
		s.traceEnabled = strings.EqualFold(value, "true")
	}
	s.engine.SetOptions(s.opts)
	return nil
}

func (s *Server) refreshProviderLocked() {
	switch strings.ToLower(s.providerKind) {
	case "openai_compatible":
		if s.endpoint == "" {
			break
		}
		s.opts.Provider = providers.OpenAICompatible{BaseURL: s.endpoint, APIKey: s.apiKey, Model: s.opts.Model, Retries: s.providerRetries}
		return
	case "anthropic":
		s.opts.Provider = providers.AnthropicProvider{BaseURL: s.endpoint, APIKey: s.apiKey, Model: s.opts.Model, Retries: s.providerRetries}
		return
	case "gemini":
		s.opts.Provider = providers.GeminiProvider{BaseURL: s.endpoint, APIKey: s.apiKey, Model: s.opts.Model, Retries: s.providerRetries}
		return
	case "ollama":
		s.opts.Provider = providers.OllamaProvider{BaseURL: s.endpoint, Model: s.opts.Model, Retries: s.providerRetries}
		return
	case "policy_prior":
		if model, err := finetune.LoadPolicyPriorModel(s.opts.Model); err == nil {
			s.opts.Provider = finetune.LocalPolicyPriorProvider{Model: model, Path: s.opts.Model}
			return
		}
	}
	s.opts.Provider = providers.MockProvider{}
}

func (s *Server) refreshVerifierLocked(enabled bool) {
	var active verifier.Verifier
	if enabled && s.verifierPath != "" {
		moveTime := s.verifierMoveTime
		if moveTime <= 0 {
			moveTime = 100
		}
		maxLoss := s.verifierMaxLoss
		if maxLoss < 0 {
			maxLoss = 180
		}
		active = verifier.ExternalUCI{Path: s.verifierPath, MoveTimeMS: moveTime, MaxCentipawnLoss: maxLoss}
	} else {
		active = verifier.StaticVerifier{Enabled: enabled}
	}
	if s.tablebaseEnabled && s.tablebasePath != "" {
		timeout := s.tablebaseTimeout
		if timeout <= 0 {
			timeout = 1000
		}
		active = verifier.TablebaseVerifier{
			Base:    active,
			Probe:   uciTablebaseProbeForPath(s.tablebasePath, timeout),
			Enabled: true,
		}
	}
	s.opts.Verifier = active
}

func (s *Server) position(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("position command missing argument")
	}
	moves := []string{}
	var fen string
	switch args[0] {
	case "startpos":
		if idx := indexOf(args, "moves"); idx >= 0 {
			moves = args[idx+1:]
		}
	case "fen":
		idx := indexOf(args, "moves")
		fenFields := args[1:]
		if idx >= 0 {
			fenFields = args[1:idx]
			moves = args[idx+1:]
		}
		if len(fenFields) < 4 {
			return fmt.Errorf("fen position requires at least 4 fields")
		}
		fen = strings.Join(fenFields, " ")
	default:
		return fmt.Errorf("unsupported position mode %s", args[0])
	}
	s.stop()
	if _, err := s.engine.NewGame(ctx, engine.NewGameOptions{Side: "auto", FEN: fen}); err != nil {
		return err
	}
	for _, mv := range moves {
		if _, err := s.engine.ApplyUserMove(ctx, mv); err != nil {
			return fmt.Errorf("invalid position move %s: %w", mv, err)
		}
	}
	return nil
}

func (s *Server) goCommand(ctx context.Context, args []string) error {
	s.mu.Lock()
	if s.searchCancel != nil {
		s.mu.Unlock()
		s.info("search already active")
		return nil
	}
	budget := parseGoBudget(args)
	moveOverhead := s.moveOverheadMS
	if moveOverhead > 0 {
		budget -= time.Duration(moveOverhead) * time.Millisecond
		if budget < 50*time.Millisecond {
			budget = 50 * time.Millisecond
		}
	}
	searchCtx, cancel := context.WithTimeout(ctx, budget)
	done := make(chan struct{})
	s.searchCancel = cancel
	s.searchDone = done
	s.mu.Unlock()

	go func() {
		defer close(done)
		defer cancel()
		dec, _, err := s.engine.ChooseMove(searchCtx)
		if err != nil {
			s.bestmove(s.bestKnownMove())
			s.clearSearch(done)
			return
		}
		if s.traceEnabled {
			_ = s.traceStore.AppendDecision(context.Background(), dec)
		}
		s.emitDecisionInfo(dec)
		s.bestmove(dec.SelectedMove.UCI)
		s.clearSearch(done)
	}()
	return nil
}

func (s *Server) bestKnownMove() string {
	moves, err := s.engine.LegalMoves(context.Background())
	if err != nil || len(moves) == 0 {
		return "0000"
	}
	return moves[0].UCI
}

func (s *Server) stop() {
	s.mu.Lock()
	cancel := s.searchCancel
	done := s.searchDone
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		_ = s.engine.Stop(context.Background())
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			s.info("stop timeout; fallback will complete asynchronously")
		}
	}
}

func (s *Server) ponderhit() {
	s.mu.Lock()
	active := s.searchCancel != nil
	s.mu.Unlock()
	if active {
		s.info("ponderhit accepted")
		return
	}
	s.info("ponderhit ignored: no active search")
}

func (s *Server) clearSearch(done chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.searchDone == done {
		s.searchCancel = nil
		s.searchDone = nil
	}
}

func (s *Server) emitDecisionInfo(dec *decision.MoveDecision) {
	if dec == nil {
		return
	}
	mode := string(dec.Mode)
	assist := "legal-only"
	if dec.Assistance.VerifierUsed {
		assist = "verifier:" + dec.Assistance.VerifierName
	}
	s.info(fmt.Sprintf("noema64 mode=%s assistance=%s fallback=%t", mode, assist, dec.FallbackUsed))
	s.info(fmt.Sprintf(
		"noema64 provider=%s model=%s prompt=%s decision_schema=%s candidates=%d",
		dec.Provider.Name,
		dec.Provider.Model,
		dec.Provider.PromptVersion,
		dec.Provider.DecisionSchemaVersion,
		len(dec.CandidateMoves),
	))
	selected := dec.SelectedMove.UCI
	if dec.SelectedMove.SAN != "" {
		selected = dec.SelectedMove.SAN + "(" + dec.SelectedMove.UCI + ")"
	}
	s.info(fmt.Sprintf(
		"noema64 selected=%s verifier=%s total_ms=%d provider_ms=%d legal_moves=%d",
		selected,
		verifierStatus(dec),
		dec.Timing.TotalMS,
		dec.Timing.ProviderMS,
		dec.LegalMovesCount,
	))
}

func verifierStatus(dec *decision.MoveDecision) string {
	if dec == nil || dec.VerifierTrace == nil {
		return "not_checked"
	}
	for _, candidate := range dec.VerifierTrace.Candidates {
		if candidate.UCI == dec.SelectedMove.UCI && candidate.Status != "" {
			return candidate.Status
		}
	}
	if dec.VerifierTrace.Used {
		return dec.VerifierTrace.Name
	}
	return "not_checked"
}

func (s *Server) write(line string) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	fmt.Fprintln(s.out, line)
}

func (s *Server) info(msg string) {
	s.write("info string " + sanitizeInfo(msg))
}

func (s *Server) bestmove(move string) {
	if move == "" {
		move = "0000"
	}
	s.write("bestmove " + move)
}

func parseSetOption(line string) (string, string) {
	fields := strings.Fields(line)
	nameParts := []string{}
	valueParts := []string{}
	readingName := false
	readingValue := false
	for _, field := range fields[1:] {
		switch field {
		case "name":
			readingName = true
			readingValue = false
		case "value":
			readingValue = true
			readingName = false
		default:
			if readingValue {
				valueParts = append(valueParts, field)
			} else if readingName {
				nameParts = append(nameParts, field)
			}
		}
	}
	return strings.Join(nameParts, " "), strings.Join(valueParts, " ")
}

func parseGoBudget(args []string) time.Duration {
	if idx := indexOf(args, "movetime"); idx >= 0 && idx+1 < len(args) {
		if ms, err := strconv.Atoi(args[idx+1]); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	if idx := indexOf(args, "depth"); idx >= 0 && idx+1 < len(args) {
		return 1500 * time.Millisecond
	}
	wtime := valueAfter(args, "wtime")
	btime := valueAfter(args, "btime")
	remaining := wtime
	if btime > 0 && (remaining == 0 || btime < remaining) {
		remaining = btime
	}
	if remaining <= 0 {
		return 3 * time.Second
	}
	budget := time.Duration(remaining/30) * time.Millisecond
	if budget < 200*time.Millisecond {
		return 200 * time.Millisecond
	}
	if budget > 12*time.Second {
		return 12 * time.Second
	}
	return budget
}

func valueAfter(args []string, key string) int {
	if idx := indexOf(args, key); idx >= 0 && idx+1 < len(args) {
		n, _ := strconv.Atoi(args[idx+1])
		return n
	}
	return 0
}

func indexOf(items []string, needle string) int {
	for i, item := range items {
		if item == needle {
			return i
		}
	}
	return -1
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func sanitizeInfo(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func engineOptions(settings storage.Settings) engine.Options {
	provider := providers.Provider(providers.MockProvider{})
	if built := uciProviderFromSettings(settings); built != nil {
		provider = built
	}
	v := verifier.Verifier(verifier.StaticVerifier{Enabled: settings.Verifier.Enabled})
	if settings.Verifier.Enabled && settings.Verifier.Path != "" {
		v = verifier.ExternalUCI{
			Path:             settings.Verifier.Path,
			MoveTimeMS:       settings.Verifier.MoveTimeMS,
			MaxCentipawnLoss: settings.Verifier.MaxCentipawnLoss,
		}
	}
	if settings.Verifier.TablebaseEnabled && settings.Verifier.TablebasePath != "" {
		v = verifier.TablebaseVerifier{
			Base:    v,
			Probe:   uciTablebaseProbeForPath(settings.Verifier.TablebasePath, settings.Verifier.TablebaseTimeoutMS),
			Enabled: true,
		}
	}
	timeout := time.Duration(settings.LLM.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	return engine.Options{
		Mode:            settings.Engine.DefaultMode,
		Personality:     settings.Engine.Personality,
		Provider:        provider,
		Verifier:        v,
		Model:           settings.LLM.Model,
		Temperature:     settings.LLM.Temperature,
		MaxTokens:       settings.LLM.MaxTokens,
		ProviderRetries: settings.LLM.Retries,
		MaxCandidates:   settings.Engine.MaxCandidates,
		MoveTimeout:     timeout,
	}
}

func uciProviderFromSettings(settings storage.Settings) providers.Provider {
	apiKey, _ := security.ResolveAPIKey(settings.LLM.APIKey, settings.LLM.APIKeyRef)
	switch settings.LLM.Provider {
	case "openai_compatible":
		if settings.LLM.Endpoint == "" {
			return nil
		}
		return providers.OpenAICompatible{BaseURL: settings.LLM.Endpoint, APIKey: apiKey, Model: settings.LLM.Model, Retries: settings.LLM.Retries}
	case "anthropic":
		return providers.AnthropicProvider{BaseURL: settings.LLM.Endpoint, APIKey: apiKey, Model: settings.LLM.Model, Retries: settings.LLM.Retries}
	case "gemini":
		return providers.GeminiProvider{BaseURL: settings.LLM.Endpoint, APIKey: apiKey, Model: settings.LLM.Model, Retries: settings.LLM.Retries}
	case "ollama":
		return providers.OllamaProvider{BaseURL: settings.LLM.Endpoint, Model: settings.LLM.Model, Retries: settings.LLM.Retries}
	case "policy_prior":
		model, err := finetune.LoadPolicyPriorModel(settings.LLM.Model)
		if err != nil {
			return nil
		}
		return finetune.LocalPolicyPriorProvider{Model: model, Path: settings.LLM.Model}
	default:
		return nil
	}
}

func uciTablebaseProbeForPath(path string, timeoutMS int) verifier.TablebaseProbe {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return verifier.NativeSyzygyProbe{Path: path}
	}
	return verifier.ExternalTablebase{Path: path, TimeoutMS: timeoutMS}
}
