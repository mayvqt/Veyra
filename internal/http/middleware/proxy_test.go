package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyUsesForwardedHeaders(t *testing.T) {
	mw := TrustedProxy([]string{"127.0.0.1/32"})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ClientIP(r) != "8.8.8.8" {
			t.Fatalf("expected forwarded ip")
		}
		if EffectiveProto(r) != "https" {
			t.Fatalf("expected forwarded proto")
		}
		if EffectiveHost(r) != "arr.example.com" {
			t.Fatalf("expected forwarded host")
		}
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "http://local/", nil)
	r.Host = "local"
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "8.8.8.8")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "arr.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestTrustedProxyAllowsForwardedHostPort(t *testing.T) {
	mw := TrustedProxy([]string{"127.0.0.1/32"})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if EffectiveHost(r) != "arr.example.com:8443" {
			t.Fatalf("expected forwarded host with port, got %q", EffectiveHost(r))
		}
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "http://local/", nil)
	r.Host = "local"
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-Host", "arr.example.com:8443")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestTrustedProxyRejectsInvalidForwardedHost(t *testing.T) {
	for _, forwardedHost := range []string{
		"https://arr.example.com",
		"arr.example.com/path",
		"arr.example.com:bad",
		"arr.example.com:70000",
		"arr example.com",
		"arr.example.com@evil.test",
	} {
		t.Run(forwardedHost, func(t *testing.T) {
			mw := TrustedProxy([]string{"127.0.0.1/32"})
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if EffectiveHost(r) != "local" {
					t.Fatalf("expected original host, got %q", EffectiveHost(r))
				}
				w.WriteHeader(http.StatusOK)
			}))

			r := httptest.NewRequest(http.MethodGet, "http://local/", nil)
			r.Host = "local"
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set("X-Forwarded-Host", forwardedHost)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("want 200 got %d", w.Code)
			}
		})
	}
}

func TestUntrustedProxyIgnoresForwardedHeaders(t *testing.T) {
	mw := TrustedProxy([]string{"10.0.0.0/8"})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ClientIP(r) != "127.0.0.1" {
			t.Fatalf("expected remote ip")
		}
		if EffectiveProto(r) != "http" {
			t.Fatalf("expected default proto")
		}
		if EffectiveHost(r) != "local" {
			t.Fatalf("expected original host")
		}
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "http://local/", nil)
	r.Host = "local"
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "8.8.8.8")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "arr.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}
