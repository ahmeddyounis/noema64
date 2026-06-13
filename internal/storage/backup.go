package storage

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const BackupManifestSchemaVersion = "backup-manifest.v1"

type BackupRequest struct {
	SettingsPath string `json:"settings_path"`
	LogDir       string `json:"log_dir"`
	OutputDir    string `json:"output_dir"`
}

type BackupManifest struct {
	SchemaVersion string       `json:"schema_version"`
	CreatedAt     string       `json:"created_at"`
	ArchivePath   string       `json:"archive_path"`
	SHA256        string       `json:"sha256"`
	Files         []BackupFile `json:"files"`
	Bytes         int64        `json:"bytes"`
}

type BackupFile struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func CreateBackup(ctx context.Context, req BackupRequest) (BackupManifest, error) {
	if strings.TrimSpace(req.OutputDir) == "" {
		req.OutputDir = "backups"
	}
	if err := os.MkdirAll(req.OutputDir, 0o700); err != nil {
		return BackupManifest{}, err
	}
	createdAt := time.Now().UTC()
	archivePath, file, err := createBackupArchive(req.OutputDir, createdAt)
	if err != nil {
		return BackupManifest{}, err
	}
	defer file.Close()
	zipWriter := zip.NewWriter(file)
	manifest := BackupManifest{
		SchemaVersion: BackupManifestSchemaVersion,
		CreatedAt:     createdAt.Format(time.RFC3339),
		ArchivePath:   archivePath,
	}
	addFile := func(sourcePath, archiveName string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !safeArchiveName(archiveName) {
			return fmt.Errorf("unsafe archive path %q", archiveName)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(archiveName)
		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer in.Close()
		n, err := io.Copy(writer, in)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, BackupFile{Path: header.Name, Bytes: n})
		manifest.Bytes += n
		return nil
	}
	if req.SettingsPath != "" {
		if err := addFile(req.SettingsPath, "config/"+filepath.Base(req.SettingsPath)); err != nil {
			return BackupManifest{}, closeZipWithError(zipWriter, file, archivePath, err)
		}
	}
	if strings.TrimSpace(req.LogDir) != "" {
		err := filepath.WalkDir(req.LogDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && sameCleanPath(path, req.OutputDir) {
				return filepath.SkipDir
			}
			if entry.IsDir() {
				return nil
			}
			if sameCleanPath(path, archivePath) {
				return nil
			}
			rel, err := filepath.Rel(req.LogDir, path)
			if err != nil {
				return err
			}
			return addFile(path, filepath.Join("logs", rel))
		})
		if err != nil {
			return BackupManifest{}, closeZipWithError(zipWriter, file, archivePath, err)
		}
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BackupManifest{}, closeZipWithError(zipWriter, file, archivePath, err)
	}
	writer, err := zipWriter.Create("manifest.json")
	if err != nil {
		return BackupManifest{}, closeZipWithError(zipWriter, file, archivePath, err)
	}
	if _, err := writer.Write(append(manifestBytes, '\n')); err != nil {
		return BackupManifest{}, closeZipWithError(zipWriter, file, archivePath, err)
	}
	if err := zipWriter.Close(); err != nil {
		_ = os.Remove(archivePath)
		return BackupManifest{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(archivePath)
		return BackupManifest{}, err
	}
	sum, err := fileSHA256(archivePath)
	if err != nil {
		return BackupManifest{}, err
	}
	manifest.SHA256 = sum
	return manifest, nil
}

func createBackupArchive(outputDir string, createdAt time.Time) (string, *os.File, error) {
	stamp := createdAt.Format("20060102T150405.000000000Z")
	for attempt := 0; attempt < 100; attempt++ {
		suffix := ""
		if attempt > 0 {
			suffix = fmt.Sprintf("-%02d", attempt)
		}
		archivePath := filepath.Join(outputDir, "noema64-backup-"+stamp+suffix+".zip")
		file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return archivePath, file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("could not create unique backup archive for timestamp %s", stamp)
}

func RestoreBackup(ctx context.Context, archivePath string, targetDir string) (BackupManifest, error) {
	if strings.TrimSpace(targetDir) == "" {
		return BackupManifest{}, fmt.Errorf("restore target directory is required")
	}
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return BackupManifest{}, err
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return BackupManifest{}, err
	}
	defer reader.Close()
	if err := os.MkdirAll(absTargetDir, 0o700); err != nil {
		return BackupManifest{}, err
	}
	manifest := BackupManifest{SchemaVersion: BackupManifestSchemaVersion, ArchivePath: archivePath}
	for _, file := range reader.File {
		select {
		case <-ctx.Done():
			return manifest, ctx.Err()
		default:
		}
		if file.Name == "manifest.json" {
			continue
		}
		if !safeArchiveName(file.Name) {
			return manifest, fmt.Errorf("unsafe archive path %q", file.Name)
		}
		targetPath := filepath.Join(absTargetDir, filepath.FromSlash(file.Name))
		cleanTargetPath := filepath.Clean(targetPath)
		cleanTargetDir := filepath.Clean(absTargetDir)
		if cleanTargetPath != cleanTargetDir && !strings.HasPrefix(cleanTargetPath, cleanTargetDir+string(filepath.Separator)) {
			return manifest, fmt.Errorf("archive path %q escapes restore directory", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
			return manifest, err
		}
		in, err := file.Open()
		if err != nil {
			return manifest, err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			_ = in.Close()
			return manifest, err
		}
		n, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return manifest, copyErr
		}
		if closeErr != nil {
			return manifest, closeErr
		}
		manifest.Files = append(manifest.Files, BackupFile{Path: file.Name, Bytes: n})
		manifest.Bytes += n
	}
	sum, err := fileSHA256(archivePath)
	if err != nil {
		return manifest, err
	}
	manifest.SHA256 = sum
	manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return manifest, nil
}

func closeZipWithError(zipWriter *zip.Writer, file *os.File, archivePath string, err error) error {
	_ = zipWriter.Close()
	_ = file.Close()
	_ = os.Remove(archivePath)
	return err
}

func safeArchiveName(name string) bool {
	name = filepath.ToSlash(strings.TrimSpace(name))
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\x00") || strings.Contains(name, "\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sameCleanPath(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return filepath.Clean(absA) == filepath.Clean(absB)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
