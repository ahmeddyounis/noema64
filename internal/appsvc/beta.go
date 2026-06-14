package appsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/experiments"
	"github.com/ahmedyounis/noema64/internal/finetune"
	"github.com/ahmedyounis/noema64/internal/providers"
	"github.com/ahmedyounis/noema64/internal/security"
	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/strategy"
	"gopkg.in/yaml.v3"
)

const (
	postGameReviewSchemaVersion           = "post-game-review.v1"
	providerDashboardSchemaVersion        = "provider-dashboard.v1"
	providerComparisonSchemaVersion       = "provider-comparison.v1"
	providerProfilesExportVersion         = "provider-profiles.v1"
	promptTemplatePackSchemaVersion       = "prompt-template-pack.v1"
	promptTemplateValidationSchemaVersion = "prompt-template-validation.v1"
	maxProviderProfilesImportBytes        = 1 << 20
)

type PostGameReview struct {
	SchemaVersion   string                 `json:"schema_version"`
	GameID          string                 `json:"game_id"`
	Ply             int                    `json:"ply"`
	OutcomeStatus   string                 `json:"outcome_status"`
	OutcomeWinner   string                 `json:"outcome_winner,omitempty"`
	Summary         string                 `json:"summary"`
	SelectedMove    string                 `json:"selected_move,omitempty"`
	PositionSummary string                 `json:"position_summary,omitempty"`
	Plan            string                 `json:"plan,omitempty"`
	PlanStatus      string                 `json:"plan_status,omitempty"`
	FallbackUsed    bool                   `json:"fallback_used"`
	CandidateCount  int                    `json:"candidate_count"`
	Provider        string                 `json:"provider,omitempty"`
	Mode            strategy.EngineMode    `json:"mode,omitempty"`
	StrategyMetrics strategy.MemoryMetrics `json:"strategy_metrics"`
	Recommendations []string               `json:"recommendations"`
}

type ProviderDashboard struct {
	SchemaVersion  string                  `json:"schema_version"`
	GeneratedAt    string                  `json:"generated_at"`
	ActiveProfile  string                  `json:"active_profile"`
	ActiveProvider string                  `json:"active_provider"`
	ActiveModel    string                  `json:"active_model"`
	Profiles       []ProviderProfileStatus `json:"profiles"`
}

type ProviderProfileStatus struct {
	ID           string                 `json:"id"`
	Provider     string                 `json:"provider"`
	Mode         string                 `json:"mode,omitempty"`
	IntendedUse  string                 `json:"intended_use,omitempty"`
	Endpoint     string                 `json:"endpoint,omitempty"`
	Model        string                 `json:"model,omitempty"`
	TimeoutMS    int                    `json:"timeout_ms"`
	Retries      int                    `json:"retries"`
	Configured   bool                   `json:"configured"`
	Healthy      bool                   `json:"healthy"`
	Status       string                 `json:"status"`
	Error        string                 `json:"error,omitempty"`
	Capabilities providers.Capabilities `json:"capabilities"`
}

type ProviderComparisonSummary struct {
	SchemaVersion    string                      `json:"schema_version"`
	GeneratedAt      string                      `json:"generated_at"`
	ProfilesCompared int                         `json:"profiles_compared"`
	Positions        []experiments.SuitePosition `json:"positions"`
	Results          []ProviderComparisonResult  `json:"results"`
}

type ProviderComparisonResult struct {
	ProfileID string                           `json:"profile_id"`
	Provider  string                           `json:"provider"`
	Model     string                           `json:"model,omitempty"`
	Status    string                           `json:"status"`
	Error     string                           `json:"error,omitempty"`
	Summary   experiments.PositionSuiteSummary `json:"summary,omitempty"`
}

type providerProfilesExport struct {
	SchemaVersion string                    `json:"schema_version" yaml:"schema_version"`
	ExportedAt    string                    `json:"exported_at" yaml:"exported_at"`
	Profiles      []storage.ProviderProfile `json:"profiles" yaml:"profiles"`
}

type PromptTemplatePack struct {
	SchemaVersion string                  `json:"schema_version"`
	Source        string                  `json:"source"`
	Manifest      strategy.PromptManifest `json:"manifest"`
	System        string                  `json:"system"`
	User          string                  `json:"user"`
	Schema        string                  `json:"schema"`
}

type PromptTemplateValidation struct {
	SchemaVersion string   `json:"schema_version"`
	Valid         bool     `json:"valid"`
	Errors        []string `json:"errors,omitempty"`
}

func (a *Application) StrategyMetrics() (strategy.MemoryMetrics, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return strategy.MemoryMetrics{}, appErr("ERR_GAME_STATE", err, true)
	}
	return state.StrategyMetrics, nil
}

func (a *Application) PostGameReview() (PostGameReview, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return PostGameReview{}, appErr("ERR_GAME_STATE", err, true)
	}
	review := PostGameReview{
		SchemaVersion:   postGameReviewSchemaVersion,
		GameID:          state.Snapshot.GameID,
		Ply:             state.Snapshot.Ply,
		OutcomeStatus:   state.Snapshot.Outcome.Status,
		OutcomeWinner:   state.Snapshot.Outcome.Winner,
		StrategyMetrics: state.StrategyMetrics,
		Plan:            state.StrategyMemory.Plan.Summary,
		PlanStatus:      state.StrategyMemory.Plan.Status,
		Recommendations: recommendationsForMetrics(state.StrategyMetrics),
	}
	dec := state.LastDecision
	if dec == nil {
		review.Summary = "No engine decision is available yet. Play, analyze, or request an engine move to generate a review."
		return review, nil
	}
	review.SelectedMove = dec.SelectedMove.UCI
	if dec.SelectedMove.SAN != "" {
		review.SelectedMove = dec.SelectedMove.SAN + " (" + dec.SelectedMove.UCI + ")"
	}
	review.PositionSummary = dec.PositionSummary
	review.FallbackUsed = dec.FallbackUsed
	review.CandidateCount = len(dec.CandidateMoves)
	review.Provider = dec.Provider.Name
	review.Mode = dec.Mode
	review.Summary = fmt.Sprintf("Last decision selected %s at ply %d. Outcome is %s. Plan status is %s.", review.SelectedMove, dec.Ply, review.OutcomeStatus, review.PlanStatus)
	if dec.FallbackUsed {
		review.Recommendations = append(review.Recommendations, "Review provider reliability because the last decision used the fallback policy.")
	}
	if dec.Assistance.VerifierUsed {
		review.Recommendations = append(review.Recommendations, "Inspect verifier output before reusing this line in analysis.")
	}
	if len(review.Recommendations) == 0 {
		review.Recommendations = append(review.Recommendations, "No immediate review issues were detected.")
	}
	return review, nil
}

func (a *Application) ProviderDashboard() (ProviderDashboard, error) {
	settings := a.settings
	dashboard := ProviderDashboard{
		SchemaVersion:  providerDashboardSchemaVersion,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ActiveProfile:  settings.LLM.ProfileID,
		ActiveProvider: settings.LLM.Provider,
		ActiveModel:    settings.LLM.Model,
	}
	for _, profile := range settings.LLM.Profiles {
		dashboard.Profiles = append(dashboard.Profiles, a.providerProfileStatus(profile))
	}
	return dashboard, nil
}

func (a *Application) ExportProviderProfiles() (string, error) {
	out := providerProfilesExport{
		SchemaVersion: providerProfilesExportVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Profiles:      redactProfiles(a.settings.LLM.Profiles),
	}
	b, err := yaml.Marshal(out)
	if err != nil {
		return "", appErr("ERR_EXPORT_PROFILES", err, true)
	}
	return string(b), nil
}

func (a *Application) ImportProviderProfiles(text string) (storage.Settings, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return storage.Settings{}, &AppError{Code: "ERR_IMPORT_PROFILES", Message: "Provider profile import is empty", Recoverable: true}
	}
	if len(text) > maxProviderProfilesImportBytes {
		return storage.Settings{}, &AppError{Code: "ERR_IMPORT_PROFILES", Message: "Provider profile import is too large", Recoverable: true}
	}
	profiles, err := parseProviderProfiles(text)
	if err != nil {
		return storage.Settings{}, appErr("ERR_IMPORT_PROFILES", err, true)
	}
	settings := cloneSettings(a.settings)
	settings.LLM.Profiles = profiles
	preserveRedactedProfileKeys(&settings, a.settings)
	settings = storage.NormalizeSettings(settings)
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return storage.Settings{}, appErr("ERR_SETTINGS_INVALID", err, true)
	}
	a.settings = storage.NormalizeSettings(settings)
	a.settingsLoadErr = nil
	a.engine.SetOptions(a.engineOptions())
	return a.GetSettings()
}

func (a *Application) SaveProviderAPIKeyToKeychain(profileID string, apiKey string) (storage.Settings, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = strings.TrimSpace(a.settings.LLM.ProfileID)
	}
	if profileID == "" || profileID == "custom" {
		profileID = "active"
	}
	keyRef := "provider/" + profileID
	if err := security.StoreKeychainSecret(keyRef, apiKey); err != nil {
		return storage.Settings{}, appErr("ERR_KEYCHAIN", err, true)
	}
	settings := cloneSettings(a.settings)
	settings.LLM.APIKey = ""
	settings.LLM.APIKeyRef = keyRef
	for i := range settings.LLM.Profiles {
		if settings.LLM.Profiles[i].ID == profileID {
			settings.LLM.Profiles[i].APIKey = ""
			settings.LLM.Profiles[i].APIKeyRef = keyRef
		}
	}
	if err := a.SaveSettings(settings); err != nil {
		return storage.Settings{}, err
	}
	return a.GetSettings()
}

func (a *Application) PromptTemplatePack() (PromptTemplatePack, error) {
	source := "default"
	templates := strategy.DefaultPromptTemplates()
	if dir := strings.TrimSpace(os.Getenv(strategy.PromptTemplateDirEnv)); dir != "" {
		loaded, err := strategy.LoadPromptTemplates(dir)
		if err != nil {
			return PromptTemplatePack{}, appErr("ERR_PROMPT_TEMPLATES", err, true)
		}
		source = dir
		templates = loaded
	}
	return promptTemplatePackFromTemplates(source, templates), nil
}

func (a *Application) ValidatePromptTemplatePack(pack PromptTemplatePack) (PromptTemplateValidation, error) {
	return validatePromptTemplatePack(pack), nil
}

func (a *Application) SavePromptTemplatePack(dir string, pack PromptTemplatePack) (PromptTemplateValidation, error) {
	validation := validatePromptTemplatePack(pack)
	if !validation.Valid {
		return validation, nil
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return PromptTemplateValidation{}, &AppError{Code: "ERR_SAVE_PROMPT_TEMPLATES", Message: "Prompt template directory is required", Recoverable: true}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return PromptTemplateValidation{}, appErr("ERR_SAVE_PROMPT_TEMPLATES", err, true)
	}
	manifest, err := json.MarshalIndent(pack.Manifest, "", "  ")
	if err != nil {
		return PromptTemplateValidation{}, appErr("ERR_SAVE_PROMPT_TEMPLATES", err, true)
	}
	files := map[string][]byte{
		"manifest.json":    append(manifest, '\n'),
		"system.md":        []byte(strings.TrimRight(pack.System, "\n") + "\n"),
		"move_decision.md": []byte(strings.TrimRight(pack.User, "\n") + "\n"),
		"schema.json":      []byte(strings.TrimSpace(pack.Schema) + "\n"),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			return PromptTemplateValidation{}, appErr("ERR_SAVE_PROMPT_TEMPLATES", err, true)
		}
	}
	return validation, nil
}

func (a *Application) RunPositionSuite(positions []experiments.SuitePosition) (experiments.PositionSuiteSummary, error) {
	opts := a.engineOptions()
	runner := experiments.Runner{Options: opts}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	summary, err := runner.PositionSuite(ctx, positions)
	return summary, appErr("ERR_EXPERIMENT", err, true)
}

func (a *Application) RunProviderComparison() (ProviderComparisonSummary, error) {
	positions := experiments.DefaultPositionSuite()
	summary := ProviderComparisonSummary{
		SchemaVersion: providerComparisonSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Positions:     positions,
	}
	for _, profile := range a.settings.LLM.Profiles {
		result := a.runProviderProfileComparison(profile, positions)
		if result.Status == "completed" {
			summary.ProfilesCompared++
		}
		summary.Results = append(summary.Results, result)
	}
	return summary, nil
}

func (a *Application) providerProfileStatus(profile storage.ProviderProfile) ProviderProfileStatus {
	provider, status, err := providerFromProfile(profile)
	out := ProviderProfileStatus{
		ID:          profile.ID,
		Provider:    profile.Provider,
		Mode:        profile.Mode,
		IntendedUse: profile.IntendedUse,
		Endpoint:    profile.Endpoint,
		Model:       profile.Model,
		TimeoutMS:   profile.TimeoutMS,
		Retries:     profile.Retries,
		Configured:  err == nil,
		Status:      status,
	}
	if provider != nil {
		out.Capabilities = provider.Capabilities()
	}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if providerRequiresPrivacyAck(profile.Provider) && !a.settings.Privacy.CloudProviderWarningAcknowledged {
		out.Status = "privacy_ack_required"
		out.Error = "Cloud/local provider data sharing must be acknowledged before health checks."
		return out
	}
	healthCtx, cancel := context.WithTimeout(context.Background(), providerHealthTimeout(profile.TimeoutMS))
	defer cancel()
	if err := provider.HealthCheck(healthCtx); err != nil {
		out.Status = "unhealthy"
		out.Error = err.Error()
		return out
	}
	out.Healthy = true
	out.Status = "healthy"
	return out
}

func (a *Application) runProviderProfileComparison(profile storage.ProviderProfile, positions []experiments.SuitePosition) ProviderComparisonResult {
	provider, status, err := providerFromProfile(profile)
	result := ProviderComparisonResult{
		ProfileID: profile.ID,
		Provider:  profile.Provider,
		Model:     profile.Model,
		Status:    status,
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if providerRequiresPrivacyAck(profile.Provider) && !a.settings.Privacy.CloudProviderWarningAcknowledged {
		result.Status = "privacy_ack_required"
		result.Error = "Cloud/local provider data sharing must be acknowledged before provider comparison."
		return result
	}
	opts := a.engineOptions()
	opts.Provider = provider
	opts.Model = profile.Model
	if profile.Temperature > 0 {
		opts.Temperature = profile.Temperature
	}
	if profile.MaxTokens > 0 {
		opts.MaxTokens = profile.MaxTokens
	}
	if profile.TimeoutMS > 0 {
		opts.MoveTimeout = time.Duration(clampComparisonTimeoutMS(profile.TimeoutMS)) * time.Millisecond
	}
	if mode := profileMode(profile.Mode); mode != "" {
		opts.Mode = mode
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(clampComparisonTimeoutMS(profile.TimeoutMS)*len(positions)+1000)*time.Millisecond)
	defer cancel()
	suite, err := (experiments.Runner{Options: opts}).PositionSuite(ctx, positions)
	result.Summary = suite
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result
	}
	if suite.EngineErrors > 0 {
		result.Status = "completed_with_errors"
		return result
	}
	result.Status = "completed"
	return result
}

func providerFromProfile(profile storage.ProviderProfile) (providers.Provider, string, error) {
	switch profile.Provider {
	case "mock", "":
		return providers.MockProvider{}, "configured", nil
	case "openai":
		apiKey, err := security.ResolveAPIKey(profile.APIKey, profile.APIKeyRef)
		if err != nil {
			return providers.OpenAIProvider{BaseURL: providers.OpenAIBaseURL, Model: profile.Model, Retries: profile.Retries}, "keychain_unavailable", err
		}
		return providers.OpenAIProvider{
			BaseURL: providers.OpenAIBaseURL,
			APIKey:  apiKey,
			Model:   profile.Model,
			Retries: profile.Retries,
		}, "configured", nil
	case "openai_compatible":
		if strings.TrimSpace(profile.Endpoint) == "" {
			return providers.OpenAICompatible{Model: profile.Model, Retries: profile.Retries}, "config_missing", fmt.Errorf("provider endpoint is required")
		}
		apiKey, err := security.ResolveAPIKey(profile.APIKey, profile.APIKeyRef)
		if err != nil {
			return providers.OpenAICompatible{BaseURL: profile.Endpoint, Model: profile.Model, Retries: profile.Retries}, "keychain_unavailable", err
		}
		return providers.OpenAICompatible{
			BaseURL: profile.Endpoint,
			APIKey:  apiKey,
			Model:   profile.Model,
			Retries: profile.Retries,
		}, "configured", nil
	case "anthropic":
		apiKey, err := security.ResolveAPIKey(profile.APIKey, profile.APIKeyRef)
		if err != nil {
			return providers.AnthropicProvider{BaseURL: profile.Endpoint, Model: profile.Model, Retries: profile.Retries}, "keychain_unavailable", err
		}
		return providers.AnthropicProvider{BaseURL: profile.Endpoint, APIKey: apiKey, Model: profile.Model, Retries: profile.Retries}, "configured", nil
	case "gemini":
		apiKey, err := security.ResolveAPIKey(profile.APIKey, profile.APIKeyRef)
		if err != nil {
			return providers.GeminiProvider{BaseURL: profile.Endpoint, Model: profile.Model, Retries: profile.Retries}, "keychain_unavailable", err
		}
		return providers.GeminiProvider{BaseURL: profile.Endpoint, APIKey: apiKey, Model: profile.Model, Retries: profile.Retries}, "configured", nil
	case "ollama":
		return providers.OllamaProvider{BaseURL: profile.Endpoint, Model: profile.Model, Retries: profile.Retries}, "configured", nil
	case "policy_prior":
		path := strings.TrimSpace(profile.Model)
		if path == "" {
			path = strings.TrimSpace(profile.Endpoint)
		}
		if path == "" {
			return nil, "config_missing", fmt.Errorf("policy_prior model path is required")
		}
		model, err := finetune.LoadPolicyPriorModel(path)
		if err != nil {
			return nil, "model_unavailable", err
		}
		return finetune.LocalPolicyPriorProvider{Model: model, Path: path}, "configured", nil
	default:
		return nil, "unsupported", fmt.Errorf("unsupported provider %q", profile.Provider)
	}
}

func providerHealthTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 || timeoutMS > 3000 {
		timeoutMS = 3000
	}
	if timeoutMS < 250 {
		timeoutMS = 250
	}
	return time.Duration(timeoutMS) * time.Millisecond
}

func clampComparisonTimeoutMS(timeoutMS int) int {
	if timeoutMS <= 0 || timeoutMS > 3000 {
		return 3000
	}
	if timeoutMS < 250 {
		return 250
	}
	return timeoutMS
}

func profileMode(mode string) strategy.EngineMode {
	switch strategy.EngineMode(strings.TrimSpace(mode)) {
	case strategy.ModePure, strategy.ModeBlunderguard, strategy.ModeHybrid, strategy.ModeCoach:
		return strategy.EngineMode(mode)
	default:
		return ""
	}
}

func redactProfiles(profiles []storage.ProviderProfile) []storage.ProviderProfile {
	out := append([]storage.ProviderProfile(nil), profiles...)
	for i := range out {
		if out[i].APIKey != "" {
			out[i].APIKey = "[REDACTED]"
		}
	}
	return out
}

func parseProviderProfiles(text string) ([]storage.ProviderProfile, error) {
	var wrapped providerProfilesExport
	if err := yaml.Unmarshal([]byte(text), &wrapped); err == nil && len(wrapped.Profiles) > 0 {
		return wrapped.Profiles, nil
	}
	var profiles []storage.ProviderProfile
	if err := yaml.Unmarshal([]byte(text), &profiles); err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("provider profile import did not contain profiles")
	}
	return profiles, nil
}

func promptTemplatePackFromTemplates(source string, templates strategy.PromptTemplates) PromptTemplatePack {
	return PromptTemplatePack{
		SchemaVersion: promptTemplatePackSchemaVersion,
		Source:        source,
		Manifest:      templates.Manifest,
		System:        templates.System,
		User:          templates.User,
		Schema:        templates.Schema,
	}
}

func validatePromptTemplatePack(pack PromptTemplatePack) PromptTemplateValidation {
	templates := strategy.PromptTemplates{
		Manifest: pack.Manifest,
		System:   pack.System,
		User:     pack.User,
		Schema:   pack.Schema,
	}
	if err := strategy.ValidatePromptTemplates(templates); err != nil {
		return PromptTemplateValidation{
			SchemaVersion: promptTemplateValidationSchemaVersion,
			Valid:         false,
			Errors:        []string{err.Error()},
		}
	}
	return PromptTemplateValidation{SchemaVersion: promptTemplateValidationSchemaVersion, Valid: true}
}

func recommendationsForMetrics(metrics strategy.MemoryMetrics) []string {
	out := []string{}
	for _, alert := range metrics.Alerts {
		switch alert.Code {
		case "strategy_drift_high", "strategy_plan_abandoned":
			out = append(out, "Review the latest plan change against candidate moves and tactical warnings.")
		case "strategy_memory_incomplete", "missing_refutation_triggers":
			out = append(out, "Run analysis to enrich strategy memory before relying on the plan.")
		case "low_strategy_confidence":
			out = append(out, "Use a stronger provider profile or a slower verifier setting for this position.")
		}
	}
	return dedupeStrings(out)
}

func dedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
