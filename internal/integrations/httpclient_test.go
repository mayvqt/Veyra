package integrations

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPClientRefusesRedirects(t *testing.T) {
	client := NewHTTPClient(time.Second)
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect error = %v, want http.ErrUseLastResponse", err)
	}
}
