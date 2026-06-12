package security

import "testing"

func TestRedactSecrets(t *testing.T) {
	in := `Authorization: Bearer sk-testsecret api_key: abc123`
	out := RedactSecrets(in)
	if out == in || out == "" {
		t.Fatalf("redaction did not change input: %q", out)
	}
	if outContains(out, "sk-testsecret") || outContains(out, "abc123") {
		t.Fatalf("secret leaked: %s", out)
	}
}

func outContains(s, needle string) bool {
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
