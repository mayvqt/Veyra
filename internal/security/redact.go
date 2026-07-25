package security

import (
	"regexp"
	"strings"
)

var credentialedURLRE = regexp.MustCompile(`(?i)://[^\s/@:]+:[^\s/@]+@`)

func RedactSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 6 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}

func RedactErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if credentialedURLRE.MatchString(msg) {
		return "redacted sensitive error"
	}
	for _, k := range []string{"token", "apikey", "api_key", "api key", "x-api-key", "authorization", "bearer", "password", "passwd", "secret"} {
		if strings.Contains(lower, k) {
			return "redacted sensitive error"
		}
	}
	return msg
}
