package auth

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

const (
	DefaultSessionDuration  = 12 * time.Hour
	RememberSessionDuration = 7 * 24 * time.Hour
	sessionTouchInterval    = 90 * time.Second
	adminCheckInterval      = 15 * time.Minute
	adminRetryInterval      = 30 * time.Second
)

type AdminStatusChecker interface {
	FetchUserAdminStatus(ctx context.Context, userID, token string) (bool, error)
}

type Service struct {
	db            *sql.DB
	crypto        *security.Crypto
	sessionSecret string
	adminChecker  AdminStatusChecker
	log           *slog.Logger
	adminRetryMu  sync.Mutex
	adminRetryAt  map[string]time.Time
}

func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

func NewService(db *sql.DB, crypto *security.Crypto, sessionSecret string, adminChecker AdminStatusChecker) *Service {
	return &Service{db: db, crypto: crypto, sessionSecret: sessionSecret, adminChecker: adminChecker, adminRetryAt: make(map[string]time.Time)}
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
		s.warnPersistence("touch session", store.TouchSession(ctx, s.db, sessRow.IDHash, now))
		sessRow.LastSeenAt = now
	}
	usrRow, err := store.GetUserByID(ctx, s.db, sessRow.UserID)
	if err != nil {
		return Session{}, User{}, err
	}
	sess := Session{IDHash: sessRow.IDHash, UserID: sessRow.UserID, MediaServerAccessTokenEncrypted: sessRow.MediaServerAccessTokenEncrypted, ExpiresAt: sessRow.ExpiresAt, CreatedAt: sessRow.CreatedAt, LastSeenAt: sessRow.LastSeenAt, AbsoluteExpiresAt: sessRow.AbsoluteExpiresAt, AdminCheckedAt: sessRow.AdminCheckedAt, IPAddress: sessRow.IPAddress, UserAgent: sessRow.UserAgent}
	usr := User{ID: usrRow.ID, MediaServerUserID: usrRow.MediaServerUserID, Username: usrRow.Username, DisplayName: usrRow.DisplayName, IsAdmin: usrRow.IsAdmin}

	if s.adminChecker != nil && adminCheckDue(sess.AdminCheckedAt, now) {
		if s.adminRetryBlocked(sess.IDHash, now) {
			usr.IsAdmin = false
			return sess, usr, nil
		}
		token, decErr := s.crypto.Decrypt(sess.MediaServerAccessTokenEncrypted)
		var checkErr error
		if decErr == nil && token != "" {
			var isAdmin bool
			isAdmin, checkErr = s.adminChecker.FetchUserAdminStatus(ctx, usr.MediaServerUserID, token)
			if checkErr == nil {
				if isAdmin != usr.IsAdmin {
					meta := "admin_changed"
					s.warnPersistence("audit permission change", store.InsertAuditLog(ctx, s.db, &usr.ID, "permission.change_detected", usr.MediaServerUserID, meta, sess.IPAddress))
				}
				usr.IsAdmin = isAdmin
				if err := store.UpdateUserAdmin(ctx, s.db, usr.ID, isAdmin); err != nil {
					return Session{}, User{}, fmt.Errorf("persist administrator status: %w", err)
				}
				s.clearAdminRetry(sess.IDHash)
				s.warnPersistence("record admin check", store.UpdateSessionAdminCheckedAt(ctx, s.db, sess.IDHash, time.Now().UTC()))
				return sess, usr, nil
			}
		} else if decErr != nil {
			checkErr = decErr
		} else {
			checkErr = fmt.Errorf("media server token is empty")
		}
		// Deny this session without turning a temporary upstream failure into a
		// durable permission change. A confirmed non-admin response is persisted
		// through the successful branch above.
		usr.IsAdmin = false
		s.setAdminRetry(sess.IDHash, now.Add(adminRetryInterval))
		if s.log != nil {
			s.log.Warn("administrator verification failed", "err", checkErr)
		}
	}

	return sess, usr, nil
}

func (s *Service) warnPersistence(operation string, err error) {
	if err != nil && s.log != nil {
		s.log.Warn("authentication persistence failed", "operation", operation, "err", err)
	}
}

func adminCheckDue(lastChecked, now time.Time) bool {
	return lastChecked.IsZero() || lastChecked.After(now) || now.Sub(lastChecked) >= adminCheckInterval
}

func (s *Service) DecryptSessionToken(enc string) (string, error) {
	return s.crypto.Decrypt(enc)
}

func (s *Service) DestroySession(ctx context.Context, rawID string) error {
	idHash := security.HashSessionID(s.sessionSecret, rawID)
	s.clearAdminRetry(idHash)
	return store.DeleteSession(ctx, s.db, idHash)
}

func (s *Service) adminRetryBlocked(idHash string, now time.Time) bool {
	s.adminRetryMu.Lock()
	defer s.adminRetryMu.Unlock()
	retryAt, ok := s.adminRetryAt[idHash]
	if !ok {
		return false
	}
	if now.Before(retryAt) {
		return true
	}
	delete(s.adminRetryAt, idHash)
	return false
}

func (s *Service) setAdminRetry(idHash string, retryAt time.Time) {
	s.adminRetryMu.Lock()
	s.adminRetryAt[idHash] = retryAt
	s.adminRetryMu.Unlock()
}

func (s *Service) clearAdminRetry(idHash string) {
	s.adminRetryMu.Lock()
	delete(s.adminRetryAt, idHash)
	s.adminRetryMu.Unlock()
}
