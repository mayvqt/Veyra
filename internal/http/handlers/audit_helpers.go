package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func auditLogViews(rows []store.AuditLogRow) []AuditLogView {
	out := make([]AuditLogView, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditLogView(row))
	}
	return out
}

func auditLogView(row store.AuditLogRow) AuditLogView {
	target := "-"
	if row.Target.Valid && strings.TrimSpace(row.Target.String) != "" {
		target = row.Target.String
	}
	ip := "-"
	if row.IPAddress.Valid && strings.TrimSpace(row.IPAddress.String) != "" {
		ip = row.IPAddress.String
	}
	userID := "-"
	if row.UserID.Valid {
		userID = fmt.Sprintf("%d", row.UserID.Int64)
	}
	metadataRaw := ""
	if row.Metadata.Valid {
		metadataRaw = strings.TrimSpace(row.Metadata.String)
	}
	metadata := auditMetadataMap(metadataRaw)
	view := AuditLogView{
		Action:    row.Action,
		Target:    target,
		Actor:     auditActor(row, target),
		IPAddress: ip,
		UserID:    userID,
		Metadata:  auditMetadataSummary(metadata),
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		Category:  auditCategory(row.Action),
		Severity:  auditSeverity(row.Action),
	}
	view.Message, view.Detail = auditMessage(row.Action, target, metadata, view.Actor, ip)
	return view
}

func auditActor(row store.AuditLogRow, target string) string {
	if row.Display.Valid && strings.TrimSpace(row.Display.String) != "" {
		display := strings.TrimSpace(row.Display.String)
		if row.Username.Valid && strings.TrimSpace(row.Username.String) != "" && strings.TrimSpace(row.Username.String) != display {
			return fmt.Sprintf("%s (%s)", display, strings.TrimSpace(row.Username.String))
		}
		return display
	}
	if row.Username.Valid && strings.TrimSpace(row.Username.String) != "" {
		return strings.TrimSpace(row.Username.String)
	}
	if row.UserID.Valid {
		return fmt.Sprintf("User #%d", row.UserID.Int64)
	}
	if target != "-" && strings.HasPrefix(row.Action, "login.") {
		return target + " (attempted)"
	}
	return "System"
}

func summarizeAuditLogs(rows []store.AuditLogRow) auditStats {
	var stats auditStats
	for _, row := range rows {
		switch row.Action {
		case "login.success":
			stats.RecentLoginCount++
		case "login.failure", "service.health.failure", "permission.change_detected":
			stats.WarningLogCount++
		}
	}
	return stats
}

func auditCategory(action string) string {
	switch {
	case strings.HasPrefix(action, "login."), action == "logout":
		return "Access"
	case strings.HasPrefix(action, "admin."):
		return "Admin"
	case strings.HasPrefix(action, "service."):
		return "Service"
	case strings.HasPrefix(action, "permission."):
		return "Security"
	default:
		return "System"
	}
}

func auditSeverity(action string) string {
	switch action {
	case "login.failure", "service.health.failure", "permission.change_detected":
		return "warn"
	default:
		return "info"
	}
}

func auditMessage(action, target string, metadata map[string]string, actor, ip string) (string, string) {
	switch action {
	case "login.success":
		return "Signed in", actor + " opened a session from " + ip
	case "login.failure":
		return "Login failed", actor + " failed to sign in from " + ip
	case "logout":
		return "Signed out", actor + " ended a session from " + ip
	case "request.created":
		media := withDefault(metadata["media_type"], "media")
		detail := fmt.Sprintf("%s requested %s %s", actor, media, target)
		if seasons := metadata["seasons"]; seasons != "" {
			detail += " seasons " + seasons
		}
		if reqID := metadata["seerr_request_id"]; reqID != "" {
			detail += " (Seerr request #" + reqID + ")"
		}
		return "Request created", detail
	case "admin.settings.updated":
		widgets := metadata["widgets"]
		if widgets == "" {
			return "Settings updated", actor + " saved portal settings"
		}
		return "Settings updated", actor + " saved portal settings; enabled widgets: " + widgets
	case "service.health.failure":
		return "Service health check failed", target + " reported " + withDefault(metadata["message"], withDefault(metadata["error"], "an error"))
	case "permission.change_detected":
		return "Permission changed", "Admin status changed for " + actor
	default:
		return auditTitle(action), withDefault(target, "System event")
	}
}

func auditTitle(action string) string {
	words := strings.Fields(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(action))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
	}
	return strings.Join(words, " ")
}

func auditMetadata(fields map[string]string) string {
	b, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func auditMetadataMap(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		for key, value := range out {
			out[key] = redactAuditValue(key, value)
		}
		return out
	}
	out["message"] = redactAuditValue("message", raw)
	return out
}

func redactAuditValue(key, value string) string {
	switch key {
	case "message", "error":
		return security.RedactErr(errors.New(value))
	default:
		return security.RedactText(value)
	}
}

func auditMetadataSummary(fields map[string]string) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, key := range []string{"media_type", "seasons", "seerr_request_id", "widgets", "reason", "message", "error"} {
		if value := strings.TrimSpace(fields[key]); value != "" {
			parts = append(parts, strings.ReplaceAll(key, "_", " ")+": "+value)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	return ""
}
