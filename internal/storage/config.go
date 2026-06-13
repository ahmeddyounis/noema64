package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
	"gopkg.in/yaml.v3"
)

const settingsSchemaVersion = "1.0"

var ErrUnsupportedSchema = errors.New("unsupported schema version")

type Settings struct {
	SchemaVersion string           `json:"schema_version" yaml:"schema_version"`
	CreatedAt     string           `json:"created_at" yaml:"created_at"`
	AppVersion    string           `json:"app_version" yaml:"app_version"`
	Engine        EngineSettings   `json:"engine" yaml:"engine"`
	LLM           LLMSettings      `json:"llm" yaml:"llm"`
	Verifier      VerifierSettings `json:"verifier" yaml:"verifier"`
	GUI           GUISettings      `json:"gui" yaml:"gui"`
	Privacy       PrivacySettings  `json:"privacy" yaml:"privacy"`
	Logging       LoggingSettings  `json:"logging" yaml:"logging"`
}

type EngineSettings struct {
	DefaultMode    strategy.EngineMode  `json:"default_mode" yaml:"default_mode"`
	Personality    strategy.Personality `json:"personality" yaml:"personality"`
	MaxCandidates  int                  `json:"max_candidates" yaml:"max_candidates"`
	FallbackPolicy string               `json:"fallback_policy" yaml:"fallback_policy"`
	TraceEnabled   bool                 `json:"trace_enabled" yaml:"trace_enabled"`
}

type LLMSettings struct {
	Provider    string            `json:"provider" yaml:"provider"`
	Endpoint    string            `json:"endpoint" yaml:"endpoint"`
	Model       string            `json:"model" yaml:"model"`
	APIKey      string            `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Temperature float64           `json:"temperature" yaml:"temperature"`
	MaxTokens   int               `json:"max_tokens" yaml:"max_tokens"`
	TimeoutMS   int               `json:"timeout_ms" yaml:"timeout_ms"`
	Retries     int               `json:"retries" yaml:"retries"`
	ProfileID   string            `json:"profile_id" yaml:"profile_id"`
	Profiles    []ProviderProfile `json:"profiles" yaml:"profiles"`
}

type ProviderProfile struct {
	ID          string  `json:"id" yaml:"id"`
	Provider    string  `json:"provider" yaml:"provider"`
	Mode        string  `json:"mode,omitempty" yaml:"mode,omitempty"`
	IntendedUse string  `json:"intended_use,omitempty" yaml:"intended_use,omitempty"`
	Endpoint    string  `json:"endpoint,omitempty" yaml:"base_url,omitempty"`
	Model       string  `json:"model,omitempty" yaml:"model,omitempty"`
	APIKey      string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	TimeoutMS   int     `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	Retries     int     `json:"retries,omitempty" yaml:"retries,omitempty"`
}

type VerifierSettings struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	Kind               string `json:"kind" yaml:"kind"`
	Path               string `json:"path" yaml:"path"`
	Depth              int    `json:"depth" yaml:"depth"`
	MoveTimeMS         int    `json:"movetime_ms" yaml:"movetime_ms"`
	MaxCentipawnLoss   int    `json:"max_centipawn_loss" yaml:"max_centipawn_loss"`
	TablebaseEnabled   bool   `json:"tablebase_enabled" yaml:"tablebase_enabled"`
	TablebasePath      string `json:"tablebase_path" yaml:"tablebase_path"`
	TablebaseTimeoutMS int    `json:"tablebase_timeout_ms" yaml:"tablebase_timeout_ms"`
}

type GUISettings struct {
	Theme            string `json:"theme" yaml:"theme"`
	ShowRawJSON      bool   `json:"show_raw_json" yaml:"show_raw_json"`
	ShowVerifierEval bool   `json:"show_verifier_eval" yaml:"show_verifier_eval"`
	BoardCoordinates bool   `json:"board_coordinates" yaml:"board_coordinates"`
	TimeControl      string `json:"time_control" yaml:"time_control"`
	ClockInitialMS   int64  `json:"clock_initial_ms" yaml:"clock_initial_ms"`
	ClockIncrementMS int64  `json:"clock_increment_ms" yaml:"clock_increment_ms"`
}

type PrivacySettings struct {
	LogRawPrompts                    bool `json:"log_raw_prompts" yaml:"log_raw_prompts"`
	LogRawLLMResponses               bool `json:"log_raw_llm_responses" yaml:"log_raw_llm_responses"`
	RedactAPIKeys                    bool `json:"redact_api_keys" yaml:"redact_api_keys"`
	CloudProviderWarningAcknowledged bool `json:"cloud_provider_warning_acknowledged" yaml:"cloud_provider_warning_acknowledged"`
}

type LoggingSettings struct {
	OutputDir string `json:"output_dir" yaml:"output_dir"`
}

func DefaultSettings() Settings {
	return Settings{
		SchemaVersion: settingsSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		AppVersion:    "0.1.0",
		Engine: EngineSettings{
			DefaultMode:    strategy.ModeBlunderguard,
			Personality:    strategy.PersonalityBalanced,
			MaxCandidates:  5,
			FallbackPolicy: "safe_heuristic",
			TraceEnabled:   true,
		},
		LLM: LLMSettings{
			Provider:    "mock",
			Model:       "mock-balanced",
			Temperature: 0.2,
			MaxTokens:   1600,
			TimeoutMS:   12000,
			Retries:     1,
			Profiles:    DefaultProviderProfiles(),
		},
		Verifier: VerifierSettings{
			Enabled:            false,
			Kind:               "static",
			MoveTimeMS:         100,
			Depth:              8,
			MaxCentipawnLoss:   180,
			TablebaseTimeoutMS: 1000,
		},
		GUI: GUISettings{
			Theme:            "system",
			ShowVerifierEval: true,
			BoardCoordinates: true,
			TimeControl:      "untimed",
			ClockInitialMS:   300000,
		},
		Privacy: PrivacySettings{
			RedactAPIKeys: true,
		},
		Logging: LoggingSettings{
			OutputDir: "logs",
		},
	}
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "noema64"), nil
}

func LoadSettings(path string) (Settings, error) {
	if path == "" {
		dir, err := ConfigDir()
		if err != nil {
			return Settings{}, err
		}
		path = filepath.Join(dir, "config.yaml")
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NormalizeSettings(DefaultSettings()), nil
	}
	if err != nil {
		return Settings{}, err
	}
	settings := DefaultSettings()
	if err := yaml.Unmarshal(b, &settings); err != nil {
		return Settings{}, err
	}
	settings = NormalizeSettings(settings)
	if err := validateSettings(settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func DefaultProviderProfiles() []ProviderProfile {
	return []ProviderProfile{
		{
			ID:          "mock-fast",
			Provider:    "mock",
			Mode:        "deterministic",
			IntendedUse: "ci_and_demo",
			Model:       "mock-balanced",
			Temperature: 0.2,
			MaxTokens:   1600,
			TimeoutMS:   12000,
			Retries:     1,
		},
		{
			ID:          "local-balanced",
			Provider:    "openai_compatible",
			Mode:        "balanced",
			IntendedUse: "local_play",
			Endpoint:    "http://localhost:11434/v1",
			Model:       "configurable",
			Temperature: 0.2,
			MaxTokens:   1600,
			TimeoutMS:   12000,
			Retries:     1,
		},
		{
			ID:          "cloud-strong",
			Provider:    "openai_compatible",
			Mode:        "quality",
			IntendedUse: "analysis_and_high_quality_strategy",
			Model:       "user_configured",
			Temperature: 0.2,
			MaxTokens:   2000,
			TimeoutMS:   20000,
			Retries:     1,
		},
	}
}

func SaveSettings(path string, settings Settings) error {
	settings = NormalizeSettings(settings)
	if err := validateSettings(settings); err != nil {
		return err
	}
	if settings.CreatedAt == "" {
		settings.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if path == "" {
		dir, err := ConfigDir()
		if err != nil {
			return err
		}
		path = filepath.Join(dir, "config.yaml")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func NormalizeSettings(settings Settings) Settings {
	defaults := DefaultSettings()
	if settings.SchemaVersion == "" {
		settings.SchemaVersion = defaults.SchemaVersion
	}
	if settings.AppVersion == "" {
		settings.AppVersion = defaults.AppVersion
	}
	if settings.Engine.DefaultMode == "" {
		settings.Engine.DefaultMode = defaults.Engine.DefaultMode
	}
	if settings.Engine.Personality == "" {
		settings.Engine.Personality = defaults.Engine.Personality
	}
	if settings.Engine.MaxCandidates <= 0 {
		settings.Engine.MaxCandidates = defaults.Engine.MaxCandidates
	}
	if settings.Engine.FallbackPolicy == "" {
		settings.Engine.FallbackPolicy = defaults.Engine.FallbackPolicy
	}
	if settings.LLM.Provider == "" {
		settings.LLM.Provider = defaults.LLM.Provider
	}
	if settings.LLM.Model == "" {
		settings.LLM.Model = defaults.LLM.Model
	}
	if len(settings.LLM.Profiles) == 0 {
		settings.LLM.Profiles = defaults.LLM.Profiles
	}
	if settings.LLM.MaxTokens <= 0 {
		settings.LLM.MaxTokens = defaults.LLM.MaxTokens
	}
	if settings.LLM.TimeoutMS <= 0 {
		settings.LLM.TimeoutMS = defaults.LLM.TimeoutMS
	}
	settings.LLM = normalizeLLMProfileSelection(settings.LLM, defaults.LLM)
	if settings.Verifier.Kind == "" {
		settings.Verifier.Kind = defaults.Verifier.Kind
	}
	if settings.Verifier.MoveTimeMS <= 0 {
		settings.Verifier.MoveTimeMS = defaults.Verifier.MoveTimeMS
	}
	if settings.Verifier.Depth <= 0 {
		settings.Verifier.Depth = defaults.Verifier.Depth
	}
	if settings.Verifier.MaxCentipawnLoss <= 0 {
		settings.Verifier.MaxCentipawnLoss = defaults.Verifier.MaxCentipawnLoss
	}
	if settings.Verifier.TablebaseTimeoutMS <= 0 {
		settings.Verifier.TablebaseTimeoutMS = defaults.Verifier.TablebaseTimeoutMS
	}
	if settings.GUI.Theme == "" {
		settings.GUI.Theme = defaults.GUI.Theme
	}
	if settings.GUI.TimeControl == "" {
		settings.GUI.TimeControl = defaults.GUI.TimeControl
	}
	if settings.GUI.ClockInitialMS <= 0 {
		settings.GUI.ClockInitialMS = defaults.GUI.ClockInitialMS
	}
	if settings.GUI.ClockIncrementMS < 0 {
		settings.GUI.ClockIncrementMS = defaults.GUI.ClockIncrementMS
	}
	if settings.Logging.OutputDir == "" {
		settings.Logging.OutputDir = defaults.Logging.OutputDir
	}
	return settings
}

func normalizeLLMProfileSelection(llm LLMSettings, defaults LLMSettings) LLMSettings {
	llm.Profiles = normalizeProviderProfiles(llm.Profiles, defaults)
	if llm.ProfileID == "" {
		llm.ProfileID = inferProviderProfileID(llm, llm.Profiles)
		return llm
	}
	if llm.ProfileID == "custom" {
		return llm
	}
	if _, ok := findProviderProfile(llm.Profiles, llm.ProfileID); !ok {
		llm.ProfileID = "custom"
	}
	return llm
}

func normalizeProviderProfiles(profiles []ProviderProfile, defaults LLMSettings) []ProviderProfile {
	if len(profiles) == 0 {
		profiles = defaults.Profiles
	}
	out := make([]ProviderProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Provider == "" {
			profile.Provider = defaults.Provider
		}
		if profile.Model == "" {
			profile.Model = defaults.Model
		}
		if profile.MaxTokens <= 0 {
			profile.MaxTokens = defaults.MaxTokens
		}
		if profile.TimeoutMS <= 0 {
			profile.TimeoutMS = defaults.TimeoutMS
		}
		if profile.Retries < 0 {
			profile.Retries = defaults.Retries
		}
		out = append(out, profile)
	}
	return out
}

func inferProviderProfileID(llm LLMSettings, profiles []ProviderProfile) string {
	for _, profile := range profiles {
		if profileMatchesLLM(profile, llm) {
			return profile.ID
		}
	}
	return "custom"
}

func profileMatchesLLM(profile ProviderProfile, llm LLMSettings) bool {
	if profile.ID == "" || profile.Provider != llm.Provider {
		return false
	}
	if profile.Endpoint != "" && profile.Endpoint != llm.Endpoint {
		return false
	}
	if profile.Model != "" && profile.Model != llm.Model {
		return false
	}
	return true
}

func findProviderProfile(profiles []ProviderProfile, id string) (ProviderProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return ProviderProfile{}, false
}

func validateSettings(settings Settings) error {
	if settings.SchemaVersion == "" {
		return errors.New("settings schema_version is required")
	}
	if settings.SchemaVersion != settingsSchemaVersion {
		return fmt.Errorf("%w: settings schema_version %q is unsupported by this release", ErrUnsupportedSchema, settings.SchemaVersion)
	}
	switch settings.Engine.DefaultMode {
	case "pure", "blunderguard", "hybrid", "coach":
	default:
		return errors.New("settings engine.default_mode is invalid")
	}
	switch settings.Engine.Personality {
	case "balanced", "aggressive", "positional", "beginner_coach":
	default:
		return errors.New("settings engine.personality is invalid")
	}
	switch settings.LLM.Provider {
	case "mock", "openai_compatible":
	default:
		return errors.New("settings llm.provider is invalid")
	}
	if settings.LLM.Provider == "openai_compatible" && settings.LLM.Endpoint == "" {
		return errors.New("settings llm.endpoint is required for openai_compatible provider")
	}
	if settings.Engine.MaxCandidates < 1 || settings.Engine.MaxCandidates > 10 {
		return errors.New("settings engine.max_candidates must be between 1 and 10")
	}
	if settings.LLM.Temperature < 0 || settings.LLM.Temperature > 2 {
		return errors.New("settings llm.temperature must be between 0 and 2")
	}
	if settings.LLM.TimeoutMS < 100 || settings.LLM.TimeoutMS > 120000 {
		return errors.New("settings llm.timeout_ms must be between 100 and 120000")
	}
	if err := validateProviderProfiles(settings.LLM.Profiles); err != nil {
		return err
	}
	if settings.Verifier.MoveTimeMS < 10 || settings.Verifier.MoveTimeMS > 5000 {
		return errors.New("settings verifier.movetime_ms must be between 10 and 5000")
	}
	if settings.Verifier.MaxCentipawnLoss < 0 || settings.Verifier.MaxCentipawnLoss > 2000 {
		return errors.New("settings verifier.max_centipawn_loss must be between 0 and 2000")
	}
	if settings.Verifier.TablebaseEnabled && settings.Verifier.TablebasePath == "" {
		return errors.New("settings verifier.tablebase_path is required when tablebase is enabled")
	}
	if settings.Verifier.TablebaseTimeoutMS < 50 || settings.Verifier.TablebaseTimeoutMS > 10000 {
		return errors.New("settings verifier.tablebase_timeout_ms must be between 50 and 10000")
	}
	switch settings.GUI.TimeControl {
	case "untimed", "bullet", "blitz", "rapid", "classical", "custom":
	default:
		return errors.New("settings gui.time_control is invalid")
	}
	return nil
}

func validateProviderProfiles(profiles []ProviderProfile) error {
	seen := map[string]struct{}{}
	for _, profile := range profiles {
		if profile.ID == "" {
			return errors.New("settings llm.profiles.id is required")
		}
		if _, ok := seen[profile.ID]; ok {
			return errors.New("settings llm.profiles.id must be unique")
		}
		seen[profile.ID] = struct{}{}
		switch profile.Provider {
		case "mock", "openai_compatible":
		default:
			return errors.New("settings llm.profiles.provider is invalid")
		}
		if profile.Temperature < 0 || profile.Temperature > 2 {
			return errors.New("settings llm.profiles.temperature must be between 0 and 2")
		}
		if profile.MaxTokens < 1 {
			return errors.New("settings llm.profiles.max_tokens must be positive")
		}
		if profile.TimeoutMS < 100 || profile.TimeoutMS > 120000 {
			return errors.New("settings llm.profiles.timeout_ms must be between 100 and 120000")
		}
	}
	return nil
}
