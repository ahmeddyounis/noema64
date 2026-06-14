package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateExternalCommandAllowsSimplePATHBinary(t *testing.T) {
	path, err := ValidateExternalCommand("stockfish", DefaultExternalCommandPolicy())
	if err != nil {
		t.Fatalf("validate PATH binary: %v", err)
	}
	if path != "stockfish" {
		t.Fatalf("path = %q, want stockfish", path)
	}
}

func TestValidateExternalCommandRejectsShellLikeInput(t *testing.T) {
	for _, path := range []string{"stockfish --bad", "../stockfish", "bin/stockfish", "stockfish\nuci"} {
		if _, err := ValidateExternalCommand(path, DefaultExternalCommandPolicy()); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}

func TestValidateExternalCommandChecksAbsoluteExecutable(t *testing.T) {
	dir := t.TempDir()
	nonExecutablePath := filepath.Join(dir, "engine")
	if runtime.GOOS == "windows" {
		nonExecutablePath = filepath.Join(dir, "engine.txt")
	}
	if err := os.WriteFile(nonExecutablePath, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if _, err := ValidateExternalCommand(nonExecutablePath, DefaultExternalCommandPolicy()); err == nil {
		t.Fatal("expected non-executable file to fail")
	}
	path := filepath.Join(dir, "engine")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("write executable command: %v", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatalf("chmod command: %v", err)
	}
	validated, err := ValidateExternalCommand(path, ExternalCommandPolicy{AllowedDirs: []string{dir}})
	if err != nil {
		t.Fatalf("validate executable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve command: %v", err)
	}
	if validated != resolved {
		t.Fatalf("validated path = %q, want %q", validated, resolved)
	}
	if _, err := ValidateExternalCommand(path, ExternalCommandPolicy{AllowedDirs: []string{filepath.Join(dir, "other")}}); err == nil {
		t.Fatal("expected executable outside allowed dirs to fail")
	}
}
