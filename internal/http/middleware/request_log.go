package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

type requestLogKey string

const requestLogAttrsKey requestLogKey = "request_log_attrs"

type requestLogAttrs struct {
	username string
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func RequestLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx, attrs := withRequestLogAttrs(r.Context())
			lrw := &loggingResponseWriter{ResponseWriter: w}
			next.ServeHTTP(lrw, r.WithContext(ctx))
			if lrw.status == 0 {
				lrw.status = http.StatusOK
			}
			log.Info("http.request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", lrw.status,
				"bytes", lrw.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote", ClientIP(r),
				"username", withDefaultUsername(attrs.username),
			)
		})
	}
}

func withRequestLogAttrs(ctx context.Context) (context.Context, *requestLogAttrs) {
	if attrs, ok := ctx.Value(requestLogAttrsKey).(*requestLogAttrs); ok && attrs != nil {
		return ctx, attrs
	}
	attrs := &requestLogAttrs{}
	return context.WithValue(ctx, requestLogAttrsKey, attrs), attrs
}

func setRequestLogUsername(ctx context.Context, username string) {
	attrs, ok := ctx.Value(requestLogAttrsKey).(*requestLogAttrs)
	if !ok || attrs == nil {
		return
	}
	attrs.username = strings.TrimSpace(username)
}

func withDefaultUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "-"
	}
	return username
}
