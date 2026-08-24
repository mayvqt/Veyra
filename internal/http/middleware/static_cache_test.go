package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheStaticAssets(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	for _, test := range []struct {
		url  string
		want string
	}{
		{url: "/static/app.css?v=0.1.4", want: "public, max-age=31536000, immutable"},
		{url: "/static/app.css?v=old", want: "no-cache"},
		{url: "/static/logo.png", want: "public, max-age=3600"},
	} {
		recorder := httptest.NewRecorder()
		CacheStaticAssets("0.1.4")(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.url, nil))
		if got := recorder.Header().Get("Cache-Control"); got != test.want {
			t.Fatalf("%s: expected %q, got %q", test.url, test.want, got)
		}
	}
}
