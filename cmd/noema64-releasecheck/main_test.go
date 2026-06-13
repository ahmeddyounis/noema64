package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckArtifactReportsChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("noema64"), 0o755); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	report, err := checkArtifact(path, false)
	if err != nil {
		t.Fatalf("check artifact: %v", err)
	}
	if !strings.Contains(report, path) || !strings.Contains(report, "sha256=") {
		t.Fatalf("unexpected report: %s", report)
	}
}

func TestCheckArtifactRejectsDirectory(t *testing.T) {
	if _, err := checkArtifact(t.TempDir(), false); err == nil {
		t.Fatal("expected directory artifact to fail")
	}
}
