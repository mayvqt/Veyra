package security

import (
	"errors"
	"testing"
)

func TestRedactSecret(t *testing.T) {
	if RedactSecret("") != "" {
		t.Fatal("empty should remain empty")
	}
	if RedactSecret("abc") != "***" {
		t.Fatal("short secret should be fully masked")
	}
	out := RedactSecret("abcdefghijkl")
	if out == "abcdefghijkl" {
		t.Fatal("expected redaction")
	}
}

func TestRedactErr(t *testing.T) {
	if RedactErr(errors.New("invalid api key")) != "redacted sensitive error" {
		t.Fatal("should redact sensitive message")
	}
	if RedactErr(errors.New("upstream rejected Authorization: Bearer abc123")) != "redacted sensitive error" {
		t.Fatal("authorization bearer errors should be redacted")
	}
	if RedactErr(errors.New("Get https://user:pass@example.test/api: timeout")) != "redacted sensitive error" {
		t.Fatal("credentialed urls should be redacted")
	}
	if RedactErr(errors.New("request failed with api_key=abc123")) != "redacted sensitive error" {
		t.Fatal("api_key errors should be redacted")
	}
	if RedactErr(errors.New("dial tcp timeout")) != "dial tcp timeout" {
		t.Fatal("non-sensitive message should pass through")
	}
}
