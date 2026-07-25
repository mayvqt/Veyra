package auth

import "time"

type User struct {
	ID                int64
	MediaServerUserID string
	Username          string
	DisplayName       string
	IsAdmin           bool
}

type Session struct {
	IDHash                          string
	UserID                          int64
	MediaServerAccessTokenEncrypted string
	ExpiresAt                       time.Time
	CreatedAt                       time.Time
	LastSeenAt                      time.Time
	AbsoluteExpiresAt               time.Time
	AdminCheckedAt                  time.Time
	IPAddress                       string
	UserAgent                       string
}
