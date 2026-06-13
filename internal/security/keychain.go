package security

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var ErrKeychainUnsupported = errors.New("os keychain is not supported on this platform")

func ResolveAPIKey(configValue string, keyRef string) (string, error) {
	configValue = strings.TrimSpace(configValue)
	if configValue != "" && configValue != "[REDACTED]" {
		return configValue, nil
	}
	keyRef = strings.TrimSpace(keyRef)
	if keyRef == "" {
		return "", nil
	}
	return LookupKeychainSecret(keyRef)
}

func LookupKeychainSecret(keyRef string) (string, error) {
	keyRef = strings.TrimSpace(keyRef)
	if keyRef == "" {
		return "", fmt.Errorf("keychain reference is required")
	}
	if value := strings.TrimSpace(os.Getenv(keychainEnvName(keyRef))); value != "" {
		return value, nil
	}
	if runtime.GOOS != "darwin" {
		return "", ErrKeychainUnsupported
	}
	out, err := exec.Command("security", "find-generic-password", "-s", "noema64", "-a", keyRef, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("read keychain secret %q: %w", keyRef, err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", fmt.Errorf("keychain secret %q is empty", keyRef)
	}
	return value, nil
}

func StoreKeychainSecret(keyRef string, value string) error {
	keyRef = strings.TrimSpace(keyRef)
	if keyRef == "" {
		return fmt.Errorf("keychain reference is required")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("keychain secret is required")
	}
	if runtime.GOOS != "darwin" {
		return ErrKeychainUnsupported
	}
	if err := exec.Command("security", "add-generic-password", "-U", "-s", "noema64", "-a", keyRef, "-w", value).Run(); err != nil {
		return fmt.Errorf("store keychain secret %q: %w", keyRef, err)
	}
	return nil
}

func keychainEnvName(keyRef string) string {
	var b strings.Builder
	b.WriteString("NOEMA64_KEYCHAIN_")
	for _, r := range strings.ToUpper(keyRef) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	return b.String()
}
