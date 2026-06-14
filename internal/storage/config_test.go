package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedyounis/noema64/internal/strategy"
)

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	settings := DefaultSettings()
	settings.LLM.APIKey = "secret"
	if err := SaveSettings(path, settings); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.SchemaVersion != "1.0" {
		t.Fatalf("schema = %s", loaded.SchemaVersion)
	}
	if loaded.LLM.APIKey != "secret" {
		t.Fatal("api key did not round trip locally")
	}
}

func TestSettingsRoundTripAPIKeyRefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	settings := DefaultSettings()
	settings.LLM.APIKeyRef = "provider/active"
	settings.LLM.Profiles = []ProviderProfile{{
		ID:        "cloud",
		Provider:  "openai_compatible",
		Endpoint:  "https://api.example.test/v1",
		Model:     "model",
		APIKeyRef: "provider/cloud",
		MaxTokens: 1000,
		TimeoutMS: 1000,
	}}
	if err := SaveSettings(path, settings); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.LLM.APIKeyRef != "provider/active" || loaded.LLM.Profiles[0].APIKeyRef != "provider/cloud" {
		t.Fatalf("api key refs did not round trip: %+v", loaded.LLM)
	}
}

func TestSaveSettingsForcesPrivateFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("schema_version: \"1.0\"\n"), 0o644); err != nil {
		t.Fatalf("write loose config: %v", err)
	}
	if err := SaveSettings(path, DefaultSettings()); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
}

func TestLoadSettingsMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("schema_version: \"1.0\"\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.LLM.Provider != "mock" || loaded.LLM.Model == "" || loaded.LLM.TimeoutMS == 0 {
		t.Fatalf("defaults not merged: %+v", loaded.LLM)
	}
	if loaded.GUI.TimeControl != "untimed" || loaded.GUI.ClockInitialMS == 0 {
		t.Fatalf("gui clock defaults not merged: %+v", loaded.GUI)
	}
	if loaded.LLM.ProfileID != "mock-fast" || len(loaded.LLM.Profiles) < 3 {
		t.Fatalf("provider profiles not merged: %+v", loaded.LLM)
	}
	if loaded.Verifier.TablebaseTimeoutMS != 1000 {
		t.Fatalf("tablebase timeout default = %d, want 1000", loaded.Verifier.TablebaseTimeoutMS)
	}
}

func TestSaveSettingsAllowsCurrentEngineMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	settings := DefaultSettings()
	settings.Engine.DefaultMode = strategy.ModeCurrent
	if err := SaveSettings(path, settings); err != nil {
		t.Fatalf("save current mode settings: %v", err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("load current mode settings: %v", err)
	}
	if loaded.Engine.DefaultMode != strategy.ModeCurrent {
		t.Fatalf("default_mode = %s, want %s", loaded.Engine.DefaultMode, strategy.ModeCurrent)
	}
}

func TestLoadSettingsRejectsUnknownFutureSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("schema_version: \"99.0\"\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadSettings(path); err == nil {
		t.Fatal("expected unknown future settings schema to fail")
	} else if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestSaveSettingsStoresSelectedProviderProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	settings := DefaultSettings()
	settings.Privacy.CloudProviderWarningAcknowledged = true
	settings.LLM.ProfileID = "local-test"
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = "http://localhost:11434/v1"
	settings.LLM.Model = "llama3.1"
	settings.LLM.Temperature = 0.4
	settings.LLM.MaxTokens = 1200
	settings.LLM.TimeoutMS = 9000
	settings.LLM.Retries = 2
	settings.LLM.Profiles = []ProviderProfile{{
		ID:          "local-test",
		Provider:    "openai_compatible",
		Endpoint:    "http://localhost:11434/v1",
		Model:       "llama3.1",
		Temperature: 0.4,
		MaxTokens:   1200,
		TimeoutMS:   9000,
		Retries:     2,
	}}

	if err := SaveSettings(path, settings); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.LLM.ProfileID != "local-test" {
		t.Fatalf("profile id = %q, want local-test", loaded.LLM.ProfileID)
	}
	if loaded.LLM.Provider != "openai_compatible" || loaded.LLM.Endpoint != "http://localhost:11434/v1" || loaded.LLM.Model != "llama3.1" {
		t.Fatalf("selected provider settings not stored: %+v", loaded.LLM)
	}
	if loaded.LLM.Temperature != 0.4 || loaded.LLM.MaxTokens != 1200 || loaded.LLM.TimeoutMS != 9000 || loaded.LLM.Retries != 2 {
		t.Fatalf("selected runtime settings not stored: %+v", loaded.LLM)
	}
}

func TestSaveSettingsValidatesProviderProfiles(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.ProfileID = "custom"
	settings.LLM.Profiles = []ProviderProfile{
		{ID: "dupe", Provider: "mock", Model: "mock-balanced", MaxTokens: 100, TimeoutMS: 1000},
		{ID: "dupe", Provider: "mock", Model: "mock-balanced", MaxTokens: 100, TimeoutMS: 1000},
	}
	if err := SaveSettings(filepath.Join(t.TempDir(), "config.yaml"), settings); err == nil {
		t.Fatal("expected duplicate provider profile ids to fail")
	}
}

func TestSaveSettingsValidatesProvider(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.Provider = "openai_compatible"
	settings.LLM.Endpoint = ""
	err := SaveSettings(filepath.Join(t.TempDir(), "config.yaml"), settings)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSaveSettingsAllowsOpenAIWithoutEndpoint(t *testing.T) {
	settings := DefaultSettings()
	settings.Privacy.CloudProviderWarningAcknowledged = true
	settings.LLM.Provider = "openai"
	settings.LLM.Endpoint = ""
	settings.LLM.Model = "test-model"
	if err := SaveSettings(filepath.Join(t.TempDir(), "config.yaml"), settings); err != nil {
		t.Fatalf("save openai settings without endpoint: %v", err)
	}
}

func TestSaveSettingsValidatesTimeControl(t *testing.T) {
	settings := DefaultSettings()
	settings.GUI.TimeControl = "sudden_mystery"
	err := SaveSettings(filepath.Join(t.TempDir(), "config.yaml"), settings)
	if err == nil {
		t.Fatal("expected invalid time control to fail")
	}
}

func TestSaveSettingsValidatesRuntimeRanges(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Settings)
	}{
		{
			name: "personality",
			mutate: func(settings *Settings) {
				settings.Engine.Personality = "chaotic"
			},
		},
		{
			name: "temperature",
			mutate: func(settings *Settings) {
				settings.LLM.Temperature = 3
			},
		},
		{
			name: "timeout low",
			mutate: func(settings *Settings) {
				settings.LLM.TimeoutMS = 50
			},
		},
		{
			name: "timeout high",
			mutate: func(settings *Settings) {
				settings.LLM.TimeoutMS = 120001
			},
		},
		{
			name: "verifier movetime",
			mutate: func(settings *Settings) {
				settings.Verifier.MoveTimeMS = 5001
			},
		},
		{
			name: "verifier loss",
			mutate: func(settings *Settings) {
				settings.Verifier.MaxCentipawnLoss = 2001
			},
		},
		{
			name: "tablebase path",
			mutate: func(settings *Settings) {
				settings.Verifier.TablebaseEnabled = true
				settings.Verifier.TablebasePath = ""
			},
		},
		{
			name: "tablebase timeout low",
			mutate: func(settings *Settings) {
				settings.Verifier.TablebaseTimeoutMS = 49
			},
		},
		{
			name: "tablebase timeout high",
			mutate: func(settings *Settings) {
				settings.Verifier.TablebaseTimeoutMS = 10001
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := DefaultSettings()
			tt.mutate(&settings)
			err := SaveSettings(filepath.Join(t.TempDir(), "config.yaml"), settings)
			if err == nil {
				t.Fatal("expected invalid settings to fail")
			}
		})
	}
}
