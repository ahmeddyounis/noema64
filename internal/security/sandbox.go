package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type ExternalCommandPolicy struct {
	AllowPATHLookup bool     `json:"allow_path_lookup"`
	AllowedDirs     []string `json:"allowed_dirs,omitempty"`
}

func DefaultExternalCommandPolicy() ExternalCommandPolicy {
	return ExternalCommandPolicy{AllowPATHLookup: true}
}

func ValidateExternalCommand(path string, policy ExternalCommandPolicy) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("external command path is empty")
	}
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("external command path contains a NUL byte")
	}
	for _, r := range path {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("external command path must not contain whitespace or control characters")
		}
	}
	if !filepath.IsAbs(path) {
		if strings.ContainsRune(path, filepath.Separator) || strings.Contains(path, "..") {
			return "", fmt.Errorf("external command must be an absolute path or a simple PATH binary name")
		}
		if !policy.AllowPATHLookup {
			return "", fmt.Errorf("PATH lookup is disabled for external commands")
		}
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if len(policy.AllowedDirs) > 0 && !pathWithinAnyDir(resolved, policy.AllowedDirs) {
		return "", fmt.Errorf("external command %q is outside allowed directories", resolved)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("external command %q is a directory", resolved)
	}
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("external command %q is not executable", resolved)
	}
	return resolved, nil
}

func pathWithinAnyDir(path string, dirs []string) bool {
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			resolvedDir = filepath.Clean(dir)
		}
		rel, err := filepath.Rel(resolvedDir, path)
		if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
			return true
		}
		if err == nil && rel == "." {
			return true
		}
	}
	return false
}
