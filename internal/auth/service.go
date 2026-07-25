package auth

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

const (
	DefaultSessionDuration  = 12 * time.Hour
	RememberSessionDuration = 7 * 24 * time.Hour
	sessionTouchInterval    = 90 * time.Second
)

type AdminStatusChecker interface {
	FetchUserAdminStatus(ctx context.Context, userID, token string) (bool, error)
}

type Service struct {
	db            *sql.DB
	crypto        *security.Crypto
	sessionSecret string
	adminChecker  AdminStatusChecker
}

func NewService(db *sql.DB, crypto *security.Crypto, sessionSecret string, adminChecker AdminStatusChecker) *Service {
	return &Service{db: db, crypto: crypto, sessionSecret: sessionSecret, adminChecker: adminChecker}
}

func (s *Service) CreateSession(ctx context.Context, user User, mediaserverToken string, r *http.Request) (string, error) {
	return s.CreateSessionWithDuration(ctx, user, mediaserverToken, r, DefaultSessionDuration)
}

func (s *Service) CreateSessionWithDuration(ctx context.Context, user User, mediaserverToken string, r *http.Request, duration time.Duration) (string, error) {
	rawID, err := security.NewSessionID()
	if err != nil {
		return "", err
	}
	encryptedToken, err := s.crypto.Encrypt(mediaserverToken)
	if err != nil {
		return "", err
	}
	if duration <= 0 || duration > RememberSessionDuration {
		duration = DefaultSessionDuration
	}
	now := time.Now().UTC()
	expiresAt := now.Add(duration)
	row := store.SessionRow{IDHash: security.HashSessionID(s.sessionSecret, rawID), UserID: user.ID, MediaServerAccessTokenEncrypted: encryptedToken, ExpiresAt: expiresAt, CreatedAt: now, LastSeenAt: now, AbsoluteExpiresAt: expiresAt, AdminCheckedAt: now, IPAddress: r.RemoteAddr, UserAgent: r.UserAgent()}
	if err := store.InsertSession(ctx, s.db, row); err != nil {
		return "", err
	}
	return rawID, nil
}

func (s *Service) ResolveSession(ctx context.Context, rawID string) (Session, User, error) {
	sessRow, err := store.GetSession(ctx, s.db, security.HashSessionID(s.sessionSecret, rawID))
	if err != nil {
		return Session{}, User{}, err
	}
	now := time.Now().UTC()
	if now.Sub(sessRow.LastSeenAt) >= sessionTouchInterval {
		_ = store.TouchSession(ctx, s.db, sessRow.IDHash, now)
		sessRow.LastSeenAt = now
	}
	usrRow, err := store.GetUserByID(ctx, s.db, sessRow.UserID)
	if err != nil {
		return Session{}, User{}, err
	}
	sess := Session{IDHash: sessRow.IDHash, UserID: sessRow.UserID, MediaServerAccessTokenEncrypted: sessRow.MediaServerAccessTokenEncrypted, ExpiresAt: sessRow.ExpiresAt, CreatedAt: sessRow.CreatedAt, LastSeenAt: sessRow.LastSeenAt, AbsoluteExpiresAt: sessRow.AbsoluteExpiresAt, AdminCheckedAt: sessRow.AdminCheckedAt, IPAddress: sessRow.IPAddress, UserAgent: sessRow.UserAgent}
	usr := User{ID: usrRow.ID, MediaServerUserID: usrRow.MediaServerUserID, Username: usrRow.Username, DisplayName: usrRow.DisplayName, IsAdmin: usrRow.IsAdmin}

	if s.adminChecker != nil && (sess.AdminCheckedAt.IsZero() || time.Since(sess.AdminCheckedAt) >= 15*time.Minute) {
		token, decErr := s.crypto.Decrypt(sess.MediaServerAccessTokenEncrypted)
		if decErr == nil {
			isAdmin, checkErr := s.adminChecker.FetchUserAdminStatus(ctx, usr.MediaServerUserID, token)
			if checkErr == nil {
				if isAdmin != usr.IsAdmin {
					meta := "admin_changed"
					_ = store.InsertAuditLog(ctx, s.db, &usr.ID, "permission.change_detected", usr.MediaServerUserID, meta, sess.IPAddress)
				}
				usr.IsAdmin = isAdmin
				_ = store.UpdateUserAdmin(ctx, s.db, usr.ID, isAdmin)
				_ = store.UpdateSessionAdminCheckedAt(ctx, s.db, sess.IDHash, time.Now().UTC())
			}
		}
	}

	return sess, usr, nil
}

func (s *Service) DecryptSessionToken(enc string) (string, error) {
	return s.crypto.Decrypt(enc)
}

func (s *Service) DestroySession(ctx context.Context, rawID string) error {
	return store.DeleteSession(ctx, s.db, security.HashSessionID(s.sessionSecret, rawID))
}
