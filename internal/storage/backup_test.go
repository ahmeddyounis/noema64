package storage

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndRestoreBackup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	logDir := filepath.Join(dir, "logs")
	if err := os.WriteFile(configPath, []byte("schema_version: \"1.0\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(logDir, "games"), 0o700); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "trace.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "games", "game.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write game: %v", err)
	}

	manifest, err := CreateBackup(context.Background(), BackupRequest{
		SettingsPath: configPath,
		LogDir:       logDir,
		OutputDir:    filepath.Join(dir, "backups"),
	})
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if manifest.SchemaVersion != BackupManifestSchemaVersion || manifest.ArchivePath == "" || manifest.SHA256 == "" || len(manifest.Files) != 3 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	restoreDir := filepath.Join(dir, "restore")
	restored, err := RestoreBackup(context.Background(), manifest.ArchivePath, restoreDir)
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	if len(restored.Files) != len(manifest.Files) {
		t.Fatalf("restored files = %d, want %d", len(restored.Files), len(manifest.Files))
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "logs", "games", "game.json")); err != nil {
		t.Fatalf("restored game missing: %v", err)
	}
}

func TestCreateBackupAllowsRapidRepeatedBackups(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "trace.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	req := BackupRequest{
		LogDir:    logDir,
		OutputDir: filepath.Join(dir, "backups"),
	}
	first, err := CreateBackup(context.Background(), req)
	if err != nil {
		t.Fatalf("create first backup: %v", err)
	}
	second, err := CreateBackup(context.Background(), req)
	if err != nil {
		t.Fatalf("create second backup: %v", err)
	}
	if first.ArchivePath == second.ArchivePath {
		t.Fatalf("backup archive paths collided: %q", first.ArchivePath)
	}
}

func TestRestoreBackupAllowsCurrentDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "trace.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	manifest, err := CreateBackup(context.Background(), BackupRequest{
		LogDir:    logDir,
		OutputDir: filepath.Join(dir, "backups"),
	})
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	restoreDir := filepath.Join(dir, "restore")
	if err := os.MkdirAll(restoreDir, 0o700); err != nil {
		t.Fatalf("mkdir restore: %v", err)
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(restoreDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	if _, err := RestoreBackup(context.Background(), manifest.ArchivePath, "."); err != nil {
		t.Fatalf("restore backup to current directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "logs", "trace.jsonl")); err != nil {
		t.Fatalf("restored trace missing: %v", err)
	}
}

func TestRestoreBackupRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bad.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	w, err := zw.Create("../escape")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, err := w.Write([]byte("bad")); err != nil {
		t.Fatalf("write member: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	if _, err := RestoreBackup(context.Background(), archivePath, filepath.Join(dir, "restore")); err == nil {
		t.Fatal("expected path traversal archive to fail")
	}
}

func TestSafeArchiveNameRejectsAmbiguousSegments(t *testing.T) {
	valid := []string{
		"config/config.yaml",
		"logs/games/game.json",
		"logs/trace.jsonl",
	}
	for _, name := range valid {
		if !safeArchiveName(name) {
			t.Fatalf("safeArchiveName(%q) = false, want true", name)
		}
	}
	invalid := []string{
		"",
		"/absolute",
		"../escape",
		"logs/..",
		"logs/../escape",
		"logs/./trace.jsonl",
		"logs//trace.jsonl",
		`..\escape`,
		"logs\\escape",
		"bad\x00name",
	}
	for _, name := range invalid {
		if safeArchiveName(name) {
			t.Fatalf("safeArchiveName(%q) = true, want false", name)
		}
	}
}
