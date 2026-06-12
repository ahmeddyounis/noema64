package storage

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ahmedyounis/noema64/internal/strategy"
	"gopkg.in/yaml.v3"
)

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
	Provider    string  `json:"provider" yaml:"provider"`
	Endpoint    string  `json:"endpoint" yaml:"endpoint"`
	Model       string  `json:"model" yaml:"model"`
	APIKey      string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Temperature float64 `json:"temperature" yaml:"temperature"`
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
	TimeoutMS   int     `json:"timeout_ms" yaml:"timeout_ms"`
	Retries     int     `json:"retries" yaml:"retries"`
}

type VerifierSettings struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Kind             string `json:"kind" yaml:"kind"`
	Path             string `json:"path" yaml:"path"`
	Depth            int    `json:"depth" yaml:"depth"`
	MoveTimeMS       int    `json:"movetime_ms" yaml:"movetime_ms"`
	MaxCentipawnLoss int    `json:"max_centipawn_loss" yaml:"max_centipawn_loss"`
}

type GUISettings struct {
	Theme            string `json:"theme" yaml:"theme"`
	ShowRawJSON      bool   `json:"show_raw_json" yaml:"show_raw_json"`
	ShowVerifierEval bool   `json:"show_verifier_eval" yaml:"show_verifier_eval"`
	BoardCoordinates bool   `json:"board_coordinates" yaml:"board_coordinates"`
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
		SchemaVersion: "1.0",
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
		},
		Verifier: VerifierSettings{
			Enabled:          false,
			Kind:             "static",
			MoveTimeMS:       100,
			Depth:            8,
			MaxCentipawnLoss: 180,
		},
		GUI: GUISettings{
			Theme:            "system",
			ShowVerifierEval: true,
			BoardCoordinates: true,
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
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	settings := DefaultSettings()
	if err := yaml.Unmarshal(b, &settings); err != nil {
		return Settings{}, err
	}
	if err := validateSettings(settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func SaveSettings(path string, settings Settings) error {
	settings = normalizeSettings(settings)
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
	return os.WriteFile(path, b, 0o600)
}

func normalizeSettings(settings Settings) Settings {
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
	if settings.LLM.MaxTokens <= 0 {
		settings.LLM.MaxTokens = defaults.LLM.MaxTokens
	}
	if settings.LLM.TimeoutMS <= 0 {
		settings.LLM.TimeoutMS = defaults.LLM.TimeoutMS
	}
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
	if settings.GUI.Theme == "" {
		settings.GUI.Theme = defaults.GUI.Theme
	}
	if settings.Logging.OutputDir == "" {
		settings.Logging.OutputDir = defaults.Logging.OutputDir
	}
	return settings
}

func validateSettings(settings Settings) error {
	if settings.SchemaVersion == "" {
		return errors.New("settings schema_version is required")
	}
	switch settings.Engine.DefaultMode {
	case "pure", "blunderguard", "hybrid", "coach":
	default:
		return errors.New("settings engine.default_mode is invalid")
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
	return nil
}
