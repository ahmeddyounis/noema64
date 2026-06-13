package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type moduleInfo struct {
	Path    string
	Version string
	Main    bool
	Dir     string
}

func main() {
	modules, err := readModules(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report, err := scanModules(modules)
	for _, line := range report {
		fmt.Println(line)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readModules(r io.Reader) ([]moduleInfo, error) {
	dec := json.NewDecoder(r)
	var modules []moduleInfo
	for {
		var module moduleInfo
		if err := dec.Decode(&module); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read module list: %w", err)
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func scanModules(modules []moduleInfo) ([]string, error) {
	scanned := 0
	missing := 0
	forbidden := []string{}
	for _, module := range modules {
		if strings.TrimSpace(module.Dir) == "" {
			missing++
			continue
		}
		text, ok := readLicenseText(module.Dir)
		if !ok {
			missing++
			continue
		}
		scanned++
		if hasForbiddenLicense(text) {
			forbidden = append(forbidden, moduleID(module))
		}
	}
	lines := []string{fmt.Sprintf("license-check: scanned %d module license files; %d modules had no local license file", scanned, missing)}
	if len(forbidden) == 0 {
		lines = append(lines, "license-check: no GPL/AGPL license text found in scanned bundled dependencies")
		return lines, nil
	}
	lines = append(lines, "license-check: forbidden license text found in:")
	lines = append(lines, forbidden...)
	return lines, fmt.Errorf("forbidden dependency licenses detected")
}

func readLicenseText(dir string) (string, bool) {
	patterns := []string{"LICENSE*", "COPYING*", "NOTICE*"}
	var paths []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() > 256*1024 {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		b.Write(content)
		b.WriteByte('\n')
	}
	text := b.String()
	return text, strings.TrimSpace(text) != ""
}

func hasForbiddenLicense(text string) bool {
	upper := strings.ToUpper(text)
	for _, phrase := range []string{
		"GNU AFFERO GENERAL PUBLIC LICENSE",
		"GNU GENERAL PUBLIC LICENSE",
		"AGPL-",
		"GPL-",
	} {
		if strings.Contains(upper, phrase) {
			return true
		}
	}
	return false
}

func moduleID(module moduleInfo) string {
	if module.Version == "" {
		return module.Path
	}
	return module.Path + "@" + module.Version
}
