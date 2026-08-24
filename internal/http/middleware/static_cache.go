package middleware

import "net/http"

// CacheStaticAssets keeps correctly versioned assets until their URL changes.
func CacheStaticAssets(version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedVersion := r.URL.Query().Get("v")
			switch {
			case requestedVersion == version && version != "":
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			case requestedVersion != "":
				w.Header().Set("Cache-Control", "no-cache")
			default:
				w.Header().Set("Cache-Control", "public, max-age=3600")
			}
			next.ServeHTTP(w, r)
		})
	}
}
