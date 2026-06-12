package storage

import (
	"os"
	"path/filepath"
	"testing"
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
