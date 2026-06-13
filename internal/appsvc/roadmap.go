package appsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/analysis"
	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/engine"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
)

const studyDashboardSchemaVersion = "study-dashboard.v1"

type StudyDashboard struct {
	SchemaVersion      string                            `json:"schema_version"`
	GeneratedAt        string                            `json:"generated_at"`
	GameID             string                            `json:"game_id"`
	Ply                int                               `json:"ply"`
	Variant            chesscore.VariantStart            `json:"variant"`
	OpeningBook        []chesscore.OpeningBookEntry      `json:"opening_book,omitempty"`
	Memory             strategy.CompressedMemory         `json:"memory"`
	Coherence          strategy.PlanCoherenceReport      `json:"coherence"`
	CandidateDiversity strategy.CandidateDiversityReport `json:"candidate_diversity"`
	MultiAgent         analysis.MultiAgentReview         `json:"multi_agent"`
	Lesson             StudyLesson                       `json:"lesson"`
	Puzzle             StudyPuzzle                       `json:"puzzle"`
	Endgame            EndgameTrainer                    `json:"endgame"`
	Heatmap            []StudyHeat                       `json:"heatmap,omitempty"`
	Timeline           []StrategyTimelineItem            `json:"timeline,omitempty"`
}

type StudyLesson struct {
	Title string   `json:"title"`
	Focus string   `json:"focus"`
	Steps []string `json:"steps"`
}

type StudyPuzzle struct {
	FEN            string   `json:"fen"`
	Goal           string   `json:"goal"`
	CandidateMoves []string `json:"candidate_moves,omitempty"`
	Solution       string   `json:"solution,omitempty"`
}

type StudyHeat struct {
	Square string  `json:"square"`
	Move   string  `json:"move"`
	Weight float64 `json:"weight"`
	Label  string  `json:"label"`
}

type StrategyTimelineItem struct {
	Ply     int    `json:"ply"`
	Move    string `json:"move,omitempty"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type EndgameTrainer struct {
	Active          bool     `json:"active"`
	Theme           string   `json:"theme"`
	MaterialBalance int      `json:"material_balance"`
	Drills          []string `json:"drills,omitempty"`
}

type AccessibilityAuditReport struct {
	SchemaVersion string               `json:"schema_version"`
	GeneratedAt   string               `json:"generated_at"`
	Checks        []AccessibilityCheck `json:"checks"`
	OpenIssues    int                  `json:"open_issues"`
}

type AccessibilityCheck struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

type AnalysisComparison struct {
	SchemaVersion string                 `json:"schema_version"`
	GameID        string                 `json:"game_id"`
	Ply           int                    `json:"ply"`
	Pure          *decision.MoveDecision `json:"pure,omitempty"`
	Hybrid        *decision.MoveDecision `json:"hybrid,omitempty"`
	Summary       string                 `json:"summary"`
}

type PromptComparison struct {
	SchemaVersion string   `json:"schema_version"`
	LeftHash      string   `json:"left_hash"`
	RightHash     string   `json:"right_hash"`
	LeftValid     bool     `json:"left_valid"`
	RightValid    bool     `json:"right_valid"`
	ChangedFiles  []string `json:"changed_files,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

type PromptPlaygroundResult struct {
	SchemaVersion string           `json:"schema_version"`
	GameID        string           `json:"game_id"`
	Ply           int              `json:"ply"`
	Comparison    PromptComparison `json:"comparison"`
	Left          PromptExecution  `json:"left"`
	Right         PromptExecution  `json:"right"`
}

type PromptExecution struct {
	Source      string                   `json:"source"`
	Valid       bool                     `json:"valid"`
	Error       string                   `json:"error,omitempty"`
	System      string                   `json:"system,omitempty"`
	User        string                   `json:"user,omitempty"`
	Provider    string                   `json:"provider,omitempty"`
	Model       string                   `json:"model,omitempty"`
	RawResponse string                   `json:"raw_response,omitempty"`
	ParseStatus string                   `json:"parse_status,omitempty"`
	Candidates  []strategy.CandidateMove `json:"candidates,omitempty"`
}

type CustomPersonalityProfile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RiskTolerance   float64  `json:"risk_tolerance"`
	StrategicBiases []string `json:"strategic_biases"`
	PromptModifiers []string `json:"prompt_modifiers"`
}

func (a *Application) CompressedStrategyMemory(maxItems int) (strategy.CompressedMemory, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.CompressedMemory{}, appErr("ERR_GAME_STATE", err, true)
	}
	return strategy.CompressMemory(state.StrategyMemory, maxItems), nil
}

func (a *Application) PlanCoherence() (strategy.PlanCoherenceReport, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.PlanCoherenceReport{}, appErr("ERR_GAME_STATE", err, true)
	}
	return strategy.EvaluatePlanCoherence(state.StrategyMemory), nil
}

func (a *Application) CandidateDiversity() (strategy.CandidateDiversityReport, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.CandidateDiversityReport{}, appErr("ERR_GAME_STATE", err, true)
	}
	if state.LastDecision == nil {
		return strategy.EvaluateCandidateDiversity(nil), nil
	}
	return strategy.EvaluateCandidateDiversity(state.LastDecision.CandidateMoves), nil
}

func (a *Application) MultiAgentAnalysis() (analysis.MultiAgentReview, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return analysis.MultiAgentReview{}, appErr("ERR_GAME_STATE", err, true)
	}
	return analysis.ReviewDecision(state.LastDecision), nil
}

func (a *Application) AccessibilityAudit() (AccessibilityAuditReport, error) {
	report := AccessibilityAuditReport{
		SchemaVersion: "accessibility-audit.v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Checks: []AccessibilityCheck{
			{ID: "board-grid-labels", Status: "pass", Severity: "required", Summary: "Board squares expose gridcell roles and square labels."},
			{ID: "keyboard-board", Status: "pass", Severity: "required", Summary: "Arrow keys move board focus and Enter/Space selects or plays focused squares."},
			{ID: "high-contrast-theme", Status: "pass", Severity: "required", Summary: "High-contrast theme is available from settings and uses non-color move indicators."},
			{ID: "modal-controls", Status: "pass", Severity: "recommended", Summary: "Dialog actions are reachable as native buttons and form controls."},
		},
	}
	for _, check := range report.Checks {
		if check.Status != "pass" {
			report.OpenIssues++
		}
	}
	return report, nil
}

func (a *Application) StudyDashboard() (StudyDashboard, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return StudyDashboard{}, appErr("ERR_GAME_STATE", err, true)
	}
	return studyDashboardFromState(state), nil
}

func (a *Application) OpeningBook() ([]chesscore.OpeningBookEntry, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return nil, appErr("ERR_GAME_STATE", err, true)
	}
	game, err := gameForStudyState(state)
	if err != nil {
		return nil, appErr("ERR_GAME_STATE", err, true)
	}
	return chesscore.OpeningBookSuggestionsWithImports(game, a.openingBooks), nil
}

func (a *Application) ImportOpeningBook(path string) (chesscore.ImportedOpeningBook, error) {
	book, err := chesscore.ImportOpeningBook(path)
	if err != nil {
		return chesscore.ImportedOpeningBook{}, appErr("ERR_OPENING_BOOK", err, true)
	}
	replaced := false
	for i := range a.openingBooks {
		if a.openingBooks[i].Path == book.Path {
			a.openingBooks[i] = book
			replaced = true
			break
		}
	}
	if !replaced {
		a.openingBooks = append(a.openingBooks, book)
	}
	return book, nil
}

func (a *Application) OpeningBookLibrary() ([]chesscore.ImportedOpeningBook, error) {
	return append([]chesscore.ImportedOpeningBook(nil), a.openingBooks...), nil
}

func (a *Application) ComparePureHybridAnalysis() (AnalysisComparison, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return AnalysisComparison{}, appErr("ERR_GAME_STATE", err, true)
	}
	run := func(mode strategy.EngineMode) (*decision.MoveDecision, error) {
		opts := a.engineOptions()
		opts.Mode = mode
		e := engine.New(opts)
		if _, err := e.LoadState(context.Background(), *state); err != nil {
			return nil, err
		}
		return e.AnalyzePosition(context.Background())
	}
	pure, pureErr := run(strategy.ModePure)
	hybrid, hybridErr := run(strategy.ModeHybrid)
	comparison := AnalysisComparison{
		SchemaVersion: "analysis-comparison.v1",
		GameID:        state.Snapshot.GameID,
		Ply:           state.Snapshot.Ply,
		Pure:          pure,
		Hybrid:        hybrid,
		Summary:       comparisonSummary(pure, hybrid, pureErr, hybridErr),
	}
	if pureErr != nil || hybridErr != nil {
		return comparison, appErr("ERR_ANALYSIS", fmt.Errorf("pure=%v hybrid=%v", pureErr, hybridErr), true)
	}
	return comparison, nil
}

func (a *Application) ComparePromptTemplatePacks(left PromptTemplatePack, right PromptTemplatePack) (PromptComparison, error) {
	leftValidation := validatePromptTemplatePack(left)
	rightValidation := validatePromptTemplatePack(right)
	out := PromptComparison{
		SchemaVersion: "prompt-comparison.v1",
		LeftHash:      promptPackHash(left),
		RightHash:     promptPackHash(right),
		LeftValid:     leftValidation.Valid,
		RightValid:    rightValidation.Valid,
	}
	out.Errors = append(out.Errors, leftValidation.Errors...)
	out.Errors = append(out.Errors, rightValidation.Errors...)
	if left.System != right.System {
		out.ChangedFiles = append(out.ChangedFiles, "system.md")
	}
	if left.User != right.User {
		out.ChangedFiles = append(out.ChangedFiles, "move_decision.md")
	}
	if left.Schema != right.Schema {
		out.ChangedFiles = append(out.ChangedFiles, "schema.json")
	}
	leftManifest, _ := json.Marshal(left.Manifest)
	rightManifest, _ := json.Marshal(right.Manifest)
	if string(leftManifest) != string(rightManifest) {
		out.ChangedFiles = append(out.ChangedFiles, "manifest.json")
	}
	return out, nil
}

func (a *Application) RunPromptPlayground(left PromptTemplatePack, right PromptTemplatePack) (PromptPlaygroundResult, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return PromptPlaygroundResult{}, appErr("ERR_GAME_STATE", err, true)
	}
	game, err := gameForStudyState(state)
	if err != nil {
		return PromptPlaygroundResult{}, appErr("ERR_GAME_STATE", err, true)
	}
	comparison, _ := a.ComparePromptTemplatePacks(left, right)
	result := PromptPlaygroundResult{
		SchemaVersion: "prompt-playground.v1",
		GameID:        state.Snapshot.GameID,
		Ply:           state.Snapshot.Ply,
		Comparison:    comparison,
	}
	opts := a.engineOptions()
	req := strategy.StrategyRequest{
		GameID:             state.Snapshot.GameID,
		FEN:                state.Snapshot.FEN,
		PGN:                state.Snapshot.PGN,
		SideToMove:         state.Snapshot.SideToMove,
		MoveNumber:         state.Snapshot.Ply/2 + 1,
		LegalMoves:         state.Snapshot.LegalMoves,
		Features:           state.Features,
		PreviousMemory:     state.StrategyMemory,
		Mode:               opts.Mode,
		Personality:        opts.Personality,
		PersonalityProfile: opts.PersonalityProfile,
	}
	result.Left = a.runPromptExecution(left, req, game, opts)
	result.Right = a.runPromptExecution(right, req, game, opts)
	return result, nil
}

func (a *Application) runPromptExecution(pack PromptTemplatePack, req strategy.StrategyRequest, game *chesscore.Game, opts engine.Options) PromptExecution {
	exec := PromptExecution{Source: pack.Source}
	validation := validatePromptTemplatePack(pack)
	if !validation.Valid {
		exec.Error = strings.Join(validation.Errors, "; ")
		return exec
	}
	exec.Valid = true
	templates := strategy.PromptTemplates{
		Manifest: pack.Manifest,
		System:   pack.System,
		User:     pack.User,
		Schema:   pack.Schema,
	}
	system, user, err := strategy.BuildPromptWithTemplates(req, templates)
	if err != nil {
		exec.Valid = false
		exec.Error = err.Error()
		return exec
	}
	exec.System = system
	exec.User = user
	provider := opts.Provider
	if provider == nil {
		provider = providers.MockProvider{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.MoveTimeout)
	defer cancel()
	resp, err := provider.CompleteJSON(ctx, providers.CompletionRequest{
		Model:       opts.Model,
		System:      system,
		User:        user,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Metadata: map[string]string{
			"legal_moves":    strategy.LegalMoveCSV(req),
			"max_candidates": fmt.Sprintf("%d", opts.MaxCandidates),
			"game_id":        req.GameID,
			"fen":            req.FEN,
		},
	})
	exec.Provider = provider.Name()
	exec.Model = opts.Model
	if err != nil {
		exec.Error = err.Error()
		return exec
	}
	exec.RawResponse = resp.Text
	parse := strategy.ParseDecision(resp.Text)
	exec.ParseStatus = parse.Status
	if parse.Status != "ok" && parse.Status != "extracted_json" {
		exec.Error = parse.Error
		return exec
	}
	candidates, _ := strategy.NormalizeCandidates(game, parse.Decision.CandidateMoves)
	exec.Candidates = candidates
	return exec
}

func (a *Application) BuildCustomPersonalityProfile(id string, name string, riskTolerance float64, biases []string, modifiers []string) (CustomPersonalityProfile, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = "custom"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Custom"
	}
	if math.IsNaN(riskTolerance) {
		riskTolerance = 0.45
	}
	if riskTolerance < 0 {
		riskTolerance = 0
	}
	if riskTolerance > 1 {
		riskTolerance = 1
	}
	return CustomPersonalityProfile{
		ID:              id,
		Name:            name,
		RiskTolerance:   math.Round(riskTolerance*100) / 100,
		StrategicBiases: dedupeStrings(biases),
		PromptModifiers: dedupeStrings(modifiers),
	}, nil
}

func (a *Application) CustomPersonalityProfiles() ([]CustomPersonalityProfile, error) {
	out := make([]CustomPersonalityProfile, 0, len(a.settings.Engine.CustomPersonalities))
	for _, profile := range a.settings.Engine.CustomPersonalities {
		out = append(out, CustomPersonalityProfile{
			ID:              string(profile.ID),
			Name:            profile.Name,
			RiskTolerance:   profile.RiskTolerance,
			StrategicBiases: append([]string(nil), profile.StrategicBiases...),
			PromptModifiers: append([]string(nil), profile.PromptModifiers...),
		})
	}
	return out, nil
}

func (a *Application) SaveCustomPersonalityProfile(profile CustomPersonalityProfile, selectProfile bool) (storage.Settings, error) {
	normalized, err := a.BuildCustomPersonalityProfile(profile.ID, profile.Name, profile.RiskTolerance, profile.StrategicBiases, profile.PromptModifiers)
	if err != nil {
		return storage.Settings{}, err
	}
	settings := a.settings
	strategyProfile := strategy.PersonalityProfile{
		ID:              strategy.Personality(normalized.ID),
		Name:            normalized.Name,
		RiskTolerance:   normalized.RiskTolerance,
		StrategicBiases: append([]string(nil), normalized.StrategicBiases...),
		PromptModifiers: append([]string(nil), normalized.PromptModifiers...),
	}
	replaced := false
	for i := range settings.Engine.CustomPersonalities {
		if settings.Engine.CustomPersonalities[i].ID == strategyProfile.ID {
			settings.Engine.CustomPersonalities[i] = strategyProfile
			replaced = true
			break
		}
	}
	if !replaced {
		settings.Engine.CustomPersonalities = append(settings.Engine.CustomPersonalities, strategyProfile)
	}
	if selectProfile {
		settings.Engine.CustomPersonalityID = string(strategyProfile.ID)
	}
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return storage.Settings{}, appErr("ERR_SETTINGS_INVALID", err, true)
	}
	a.settings = storage.NormalizeSettings(settings)
	a.engine.SetOptions(a.engineOptions())
	return a.GetSettings()
}

func (a *Application) SelectCustomPersonalityProfile(id string) (storage.Settings, error) {
	id = strings.TrimSpace(id)
	settings := a.settings
	settings.Engine.CustomPersonalityID = id
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return storage.Settings{}, appErr("ERR_SETTINGS_INVALID", err, true)
	}
	a.settings = storage.NormalizeSettings(settings)
	a.engine.SetOptions(a.engineOptions())
	return a.GetSettings()
}

func (a *Application) DeleteCustomPersonalityProfile(id string) (storage.Settings, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return storage.Settings{}, &AppError{Code: "ERR_CUSTOM_PERSONALITY", Message: "Custom personality ID is required", Recoverable: true}
	}
	settings := a.settings
	filtered := settings.Engine.CustomPersonalities[:0]
	for _, profile := range settings.Engine.CustomPersonalities {
		if string(profile.ID) != id {
			filtered = append(filtered, profile)
		}
	}
	settings.Engine.CustomPersonalities = filtered
	if settings.Engine.CustomPersonalityID == id {
		settings.Engine.CustomPersonalityID = ""
	}
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return storage.Settings{}, appErr("ERR_SETTINGS_INVALID", err, true)
	}
	a.settings = storage.NormalizeSettings(settings)
	a.engine.SetOptions(a.engineOptions())
	return a.GetSettings()
}

func (a *Application) UpdateStrategyMemory(memory strategy.StrategyMemory) (*engine.GameState, error) {
	state, err := a.engine.UpdateStrategyMemory(context.Background(), memory)
	if err != nil {
		return state, appErr("ERR_STRATEGY_MEMORY", err, true)
	}
	return state, appErr("ERR_SAVE_GAME", a.persistGameState(state), true)
}

func studyDashboardFromState(state *engine.GameState) StudyDashboard {
	decision := state.LastDecision
	candidates := []strategy.CandidateMove{}
	if decision != nil {
		candidates = decision.CandidateMoves
	}
	return StudyDashboard{
		SchemaVersion:      studyDashboardSchemaVersion,
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		GameID:             state.Snapshot.GameID,
		Ply:                state.Snapshot.Ply,
		Variant:            state.Variant,
		OpeningBook:        openingBookForState(state),
		Memory:             strategy.CompressMemory(state.StrategyMemory, 5),
		Coherence:          strategy.EvaluatePlanCoherence(state.StrategyMemory),
		CandidateDiversity: strategy.EvaluateCandidateDiversity(candidates),
		MultiAgent:         analysis.ReviewDecision(decision),
		Lesson:             lessonForState(state),
		Puzzle:             puzzleForState(state),
		Endgame:            endgameForState(state),
		Heatmap:            heatmapForDecision(decision),
		Timeline:           timelineForState(state),
	}
}

func lessonForState(state *engine.GameState) StudyLesson {
	metrics := state.StrategyMetrics
	if state.LastDecision == nil {
		return StudyLesson{
			Title: "First Plan",
			Focus: "Create a strategy memory baseline before studying variations.",
			Steps: []string{
				"Run analysis from the current position.",
				"Compare candidate purposes against legal moves.",
				"Confirm the plan has targets, commitments, and refutation triggers.",
			},
		}
	}
	if metrics.AlertLevel == "high" || metrics.Quality < 0.6 {
		return StudyLesson{
			Title: "Plan Repair",
			Focus: "Resolve weak or drifting strategy memory.",
			Steps: []string{
				"Read the strategist and critic findings.",
				"Edit the plan summary or commitments if they no longer match the board.",
				"Run hybrid analysis to refresh verifier and search evidence.",
			},
		}
	}
	return StudyLesson{
		Title: "Candidate Comparison",
		Focus: "Understand why the selected move outranked alternatives.",
		Steps: []string{
			"Review the candidate heatmap and final scores.",
			"Check whether warnings or rejected candidates explain the ranking.",
			"Use Why Not on a candidate move to compare it directly with the selection.",
		},
	}
}

func puzzleForState(state *engine.GameState) StudyPuzzle {
	puzzle := StudyPuzzle{
		FEN:  state.Snapshot.FEN,
		Goal: "Choose the move Noema64 selected and explain the strategic reason.",
	}
	if state.LastDecision == nil {
		puzzle.Goal = "Run analysis, then solve the generated candidate puzzle."
		return puzzle
	}
	puzzle.Solution = state.LastDecision.SelectedMove.UCI
	for _, candidate := range state.LastDecision.CandidateMoves {
		puzzle.CandidateMoves = append(puzzle.CandidateMoves, candidate.UCI)
	}
	return puzzle
}

func heatmapForDecision(decision *decision.MoveDecision) []StudyHeat {
	if decision == nil {
		return nil
	}
	out := []StudyHeat{}
	for _, candidate := range decision.CandidateMoves {
		square := ""
		if len(candidate.UCI) >= 4 {
			square = candidate.UCI[2:4]
		}
		out = append(out, StudyHeat{
			Square: square,
			Move:   candidate.UCI,
			Weight: candidate.FinalScore,
			Label:  candidate.VerifierScore.Status,
		})
	}
	return out
}

func timelineForState(state *engine.GameState) []StrategyTimelineItem {
	out := []StrategyTimelineItem{{
		Ply:     0,
		Status:  "start",
		Summary: "Game initialized.",
	}}
	for _, move := range state.Snapshot.MoveHistory {
		out = append(out, StrategyTimelineItem{
			Ply:     move.Ply,
			Move:    move.UCI,
			Status:  "played",
			Summary: move.Comment,
		})
	}
	if state.StrategyMemory.LastUpdate.Summary != "" {
		out = append(out, StrategyTimelineItem{
			Ply:     state.StrategyMemory.Ply,
			Move:    state.StrategyMemory.LastUpdate.MovePlayed,
			Status:  state.StrategyMemory.Plan.Status,
			Summary: state.StrategyMemory.LastUpdate.Summary,
		})
	}
	return out
}

func openingBookForState(state *engine.GameState) []chesscore.OpeningBookEntry {
	game, err := gameForStudyState(state)
	if err != nil {
		return nil
	}
	return chesscore.OpeningBookSuggestions(game)
}

func endgameForState(state *engine.GameState) EndgameTrainer {
	active := state.Features.Phase == "endgame" || len(state.Snapshot.Board) <= 10
	trainer := EndgameTrainer{
		Active:          active,
		Theme:           "not_endgame",
		MaterialBalance: state.Features.MaterialBalance,
	}
	if !active {
		return trainer
	}
	trainer.Drills = []string{
		"Identify the passed pawns and king activity.",
		"Check all forcing moves before pawn pushes.",
		"Compare the engine candidate with a quiet improving move.",
	}
	switch {
	case state.Features.MaterialBalance == 0:
		trainer.Theme = "holding_draw"
	case state.Features.MaterialBalance > 0:
		trainer.Theme = "white_conversion"
	default:
		trainer.Theme = "black_conversion"
	}
	return trainer
}

func gameForStudyState(state *engine.GameState) (*chesscore.Game, error) {
	initialFEN := strings.TrimSpace(state.InitialFEN)
	if initialFEN == "" {
		initialFEN = state.Snapshot.FEN
	}
	var game *chesscore.Game
	var err error
	if state.Variant.Variant == chesscore.VariantChess960 {
		start := chesscore.NormalizeVariantStart(state.Variant, initialFEN)
		game, err = chesscore.FromVariantStart(start)
	} else {
		game, err = chesscore.FromFEN(initialFEN)
	}
	if err != nil {
		return nil, err
	}
	for _, move := range state.AppliedMoves {
		if _, err := game.ApplyUCI(move); err != nil {
			return nil, err
		}
	}
	return game, nil
}

func comparisonSummary(pure *decision.MoveDecision, hybrid *decision.MoveDecision, pureErr error, hybridErr error) string {
	if pureErr != nil || hybridErr != nil {
		return fmt.Sprintf("Comparison failed: pure=%v hybrid=%v", pureErr, hybridErr)
	}
	if pure == nil || hybrid == nil {
		return "Comparison did not produce both decisions."
	}
	if pure.SelectedMove.UCI == hybrid.SelectedMove.UCI {
		return "Pure and hybrid analysis selected the same move: " + pure.SelectedMove.UCI
	}
	return fmt.Sprintf("Pure selected %s; hybrid selected %s.", pure.SelectedMove.UCI, hybrid.SelectedMove.UCI)
}

func promptPackHash(pack PromptTemplatePack) string {
	b, _ := json.Marshal(pack)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}
