package security

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	c := NewCrypto("12345678901234567890123456789012")
	enc, err := c.Encrypt("hello-token")
	if err != nil {
		t.Fatal(err)
	}
	if enc == "hello-token" || enc == "" {
		t.Fatal("expected encrypted token")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "hello-token" {
		t.Fatalf("want hello-token got %s", dec)
	}
}

func TestSessionIDAndHash(t *testing.T) {
	a, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("session ids should be unique")
	}
	if HashSessionID("session-secret-one-with-32-characters", a) == a {
		t.Fatal("hash should not equal raw id")
	}
	if HashSessionID("session-secret-one-with-32-characters", a) == HashSessionID("session-secret-two-with-32-characters", a) {
		t.Fatal("session hashes should be keyed by the session secret")
	}
	if got := HashSessionID("session-secret-one-with-32-characters", a); !strings.HasPrefix(got, sessionHashPrefix) {
		t.Fatalf("session hash %q does not identify its format", got)
	}
}
