package storage

import (
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
