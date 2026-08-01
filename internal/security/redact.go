package security

import (
	"net/url"
	"regexp"
	"strings"
)

var credentialedURLRE = regexp.MustCompile(`(?i)://[^\s/@:]+:[^\s/@]+@`)
var sensitiveAssignmentRE = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|token|password|passwd|secret|authorization)(\s*[:=]\s*)([^\s&,;]+)`)

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

// RedactText removes common credential forms and any exact configured secrets.
// It is intended as a final safety boundary before values reach logs or views.
func RedactText(s string, secrets ...string) string {
	if s == "" {
		return ""
	}
	redacted := credentialedURLRE.ReplaceAllString(s, "://***:***@")
	redacted = sensitiveAssignmentRE.ReplaceAllString(redacted, "$1$2***")
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if len(secret) < 4 {
			continue
		}
		redacted = strings.ReplaceAll(redacted, secret, "***")
	}
	return redacted
}

// RedactURL removes URL credentials and sensitive query parameter values. If
// the input is not a URL, it still applies the general text redactor.
func RedactURL(value string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return RedactText(value)
	}
	if u.User != nil {
		u.User = url.User("***")
	}
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		switch lower {
		case "apikey", "accesstoken", "token", "password", "passwd", "secret", "authorization":
			query.Set(key, "***")
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}
