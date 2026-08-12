package app

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerAppliesProductionBounds(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	srv := newHTTPServer("127.0.0.1:0", handler)
	if srv.Addr != "127.0.0.1:0" || srv.Handler == nil {
		t.Fatalf("unexpected server construction: %+v", srv)
	}
	if srv.ReadHeaderTimeout != 10*time.Second || srv.ReadTimeout != 30*time.Second || srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected server timeouts: %+v", srv)
	}
	if srv.MaxHeaderBytes != 64<<10 {
		t.Fatalf("unexpected max header bytes: %d", srv.MaxHeaderBytes)
	}
}
