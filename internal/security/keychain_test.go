package security

import "testing"

func TestResolveAPIKeyPrefersConfigValue(t *testing.T) {
	t.Setenv("NOEMA64_KEYCHAIN_CLOUD", "from-env")
	key, err := ResolveAPIKey("from-config", "cloud")
	if err != nil {
		t.Fatalf("resolve key: %v", err)
	}
	if key != "from-config" {
		t.Fatalf("key = %q, want config value", key)
	}
}

func TestResolveAPIKeyReadsEnvironmentBackedRef(t *testing.T) {
	t.Setenv("NOEMA64_KEYCHAIN_CLOUD_PROFILE", "from-env")
	key, err := ResolveAPIKey("[REDACTED]", "cloud-profile")
	if err != nil {
		t.Fatalf("resolve key ref: %v", err)
	}
	if key != "from-env" {
		t.Fatalf("key = %q, want env-backed keychain value", key)
	}
}

func TestResolveAPIKeyAllowsMissingSecret(t *testing.T) {
	key, err := ResolveAPIKey("", "")
	if err != nil {
		t.Fatalf("resolve empty key: %v", err)
	}
	if key != "" {
		t.Fatalf("key = %q, want empty", key)
	}
}
