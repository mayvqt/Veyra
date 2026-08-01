package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mayvqt/veyra/internal/security"
)

type redactingHandler struct {
	next    slog.Handler
	secrets []string
}

func newRedactingHandler(next slog.Handler, secrets []string) slog.Handler {
	return &redactingHandler{next: next, secrets: append([]string(nil), secrets...)}
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, security.RedactText(record.Message, h.secrets...), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(h.redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, h.redactAttr(attr))
	}
	return &redactingHandler{next: h.next.WithAttrs(clean), secrets: h.secrets}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name), secrets: h.secrets}
}

func (h *redactingHandler) redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if sensitiveLogKey(attr.Key) {
		attr.Value = slog.StringValue("***")
		return attr
	}
	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(security.RedactText(attr.Value.String(), h.secrets...))
	case slog.KindAny:
		if err, ok := attr.Value.Any().(error); ok {
			attr.Value = slog.StringValue(security.RedactText(security.RedactErr(err), h.secrets...))
		} else {
			attr.Value = slog.StringValue(security.RedactText(fmt.Sprint(attr.Value.Any()), h.secrets...))
		}
	case slog.KindGroup:
		group := attr.Value.Group()
		clean := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			clean = append(clean, h.redactAttr(child))
		}
		attr.Value = slog.GroupValue(clean...)
	}
	return attr
}

func sensitiveLogKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", ""), "_", ""))
	switch key {
	case "apikey", "accesstoken", "token", "password", "passwd", "secret", "authorization", "sessionid", "csrftoken", "encryptionkey":
		return true
	default:
		return false
	}
}
