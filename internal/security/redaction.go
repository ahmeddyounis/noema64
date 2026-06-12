package security

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)(api[_-]?key["'\s:=]+)[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{12,}`),
}

func RedactSecrets(s string) string {
	out := s
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllStringFunc(out, func(match string) string {
			if strings.HasPrefix(strings.ToLower(match), "authorization") {
				return "Authorization: Bearer [REDACTED]"
			}
			if strings.HasPrefix(strings.ToLower(match), "api") {
				return regexp.MustCompile(`(?i)(api[_-]?key["'\s:=]+)`).FindString(match) + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return out
}
