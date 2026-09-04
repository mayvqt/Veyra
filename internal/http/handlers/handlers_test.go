package handlers

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/integrations/arr"
	jf "github.com/mayvqt/veyra/internal/integrations/mediaserver"
	sr "github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func mkDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func mkHandlers(t *testing.T) (*Handlers, *sql.DB) {
	t.Helper()
	db := mkDB(t)
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{AppName: "Veyra", AppBaseURL: "https://veyra.local", EncryptionKey: "12345678901234567890123456789012", CookieSecure: false, MediaServerType: config.MediaServerJellyfin, MediaServerURL: "http://jf.local", MediaServerPublicURL: "https://jf.public", MediaServerAPIKey: "jf-key", SeerrURL: "http://se.local", SeerrPublicURL: "https://se.public", SeerrAPIKey: "seerr-key", SonarrURL: "http://sonarr.local", SonarrAPIKey: "sonarr-key", RadarrURL: "http://radarr.local", RadarrAPIKey: "radarr-key", ProwlarrURL: "http://prowlarr.local", ProwlarrAPIKey: "prowlarr-key"}
	mediaClient, err := jf.NewClient(cfg.MediaServerType, cfg.MediaServerURL, cfg.MediaServerPublicURL, "")
	if err != nil {
		t.Fatal(err)
	}
	h := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), db, tmpl, auth.NewService(db, security.NewCrypto("12345678901234567890123456789012"), "session-secret-with-at-least-32-characters", nil), mediaClient, sr.NewClient(cfg.SeerrURL, cfg.SeerrPublicURL, "k"), arr.NewClient("Sonarr", cfg.SonarrURL, "k"), arr.NewClient("Radarr", cfg.RadarrURL, "k"), arr.NewProwlarrClient(cfg.ProwlarrURL, "k"), middleware.NewLoginRateLimiter(10, time.Minute))
	h.restart = func() {}
	return h, db
}

func TestLoginGetSetsCSRF(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	h.Login(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if len(w.Result().Cookies()) == 0 {
		t.Fatal("expected csrf cookie")
	}
}

func TestLogoutRequiresCSRF(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	h.LogoutPost(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d", w.Code)
	}
}

func TestLoginFailureWritesAuditLog(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	h.Login(w, r)
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie")
	}
	form := url.Values{}
	form.Set("csrf_token", cookies[0].Value)
	form.Set("username", "baduser")
	form.Set("password", "badpass")

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2.AddCookie(cookies[0])
	h.LoginPost(w2, r2)

	logs, err := store.ListRecentAuditLogsByActionTarget(context.Background(), db, "login.failure", "baduser", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Fatal("expected login.failure audit log")
	}
}

func TestGuideRendersLinks(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/guide", nil)
	h.Guide(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Open Jellyfin") {
		t.Fatal("expected guide to include Jellyfin action")
	}
	if !strings.Contains(body, "Open Seerr") {
		t.Fatal("expected guide to include Seerr action")
	}
	if !strings.Contains(body, `<nav class="topnav" aria-label="Primary navigation">`) {
		t.Fatal("expected guide navigation to have a clear accessible label")
	}
}

func TestAdminSettingsPostSavesRequestToggle(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	get := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	h.AdminSettingsGet(get, getReq)
	cookies := get.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie")
	}

	form := url.Values{}
	form.Set("csrf_token", cookies[0].Value)
	form.Set("app_name", "Veyra")
	form.Set("accent_color", "#d43f24")
	form.Set("media_server_public_url", "https://jf.public")
	form.Set("seerr_public_url", "https://se.public")
	form.Set("show_recent_media", "on")
	form.Set("show_recent_requests", "on")
	form.Set("show_request_quota", "on")
	form.Set("show_download_queue", "on")
	form.Set("show_upcoming_calendar", "on")

	post := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(cookies[0])
	h.AdminSettingsPost(post, postReq)
	if post.Code != http.StatusFound {
		t.Fatalf("want redirect got %d: %s", post.Code, post.Body.String())
	}
	value, err := store.GetSetting(context.Background(), db, settingWidgetRequestBot)
	if err != nil {
		t.Fatal(err)
	}
	if value != "false" {
		t.Fatalf("expected request toggle to save false when unchecked, got %q", value)
	}
}

func TestAdminSettingsRendersStoredSetupConfig(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	if err := config.SaveSetup(context.Background(), db, security.NewCrypto(h.cfg.EncryptionKey), config.SetupInput{
		AppBaseURL:           "https://veyra.setup",
		MediaServerType:      config.MediaServerJellyfin.String(),
		MediaServerURL:       "http://mediaserver:8096",
		MediaServerPublicURL: "https://mediaserver.setup",
		MediaServerAPIKey:    "jf-secret",
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	h.AdminSettingsGet(w, r)
	body := w.Body.String()
	assertTextOrder(t, body, "<h2>Dashboard</h2>", "<h2>Dashboard widgets</h2>", "<h2>Branding</h2>", "<h2>Dashboard links</h2>", "<h2>Service settings</h2>")
	assertTextOrder(t, body, "<h3>Media Server</h3>", `name="setup_media_server_type"`, `name="setup_media_server_url"`, `name="setup_media_server_api_key"`)
	if !strings.Contains(body, "Service settings") {
		t.Fatal("expected service settings section")
	}
	if !strings.Contains(body, `name="setup_media_server_url" value="http://mediaserver:8096"`) {
		t.Fatal("expected editable setup media server URL")
	}
	if strings.Contains(body, "jf-secret") {
		t.Fatal("settings page must not render stored API keys")
	}
}

func assertTextOrder(t *testing.T, text string, values ...string) {
	t.Helper()
	previous := -1
	for _, value := range values {
		current := strings.Index(text, value)
		if current < 0 {
			t.Fatalf("expected %q in response", value)
		}
		if current <= previous {
			t.Fatalf("expected %q to appear after previous value", value)
		}
		previous = current
	}
}

func TestAdminSettingsMarksEnvManagedSetupConfigReadOnly(t *testing.T) {
	t.Setenv("MEDIA_SERVER_URL", "http://env-mediaserver:8096")
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.MediaServerURL = "http://env-mediaserver:8096"
	if err := config.SaveSetup(context.Background(), db, security.NewCrypto(h.cfg.EncryptionKey), config.SetupInput{
		AppBaseURL:           "https://veyra.setup",
		MediaServerType:      config.MediaServerJellyfin.String(),
		MediaServerURL:       "http://stored-mediaserver:8096",
		MediaServerPublicURL: "https://mediaserver.setup",
		MediaServerAPIKey:    "jf-secret",
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	h.AdminSettingsGet(w, r)
	body := w.Body.String()
	if !strings.Contains(body, `name="setup_media_server_url" value="http://env-mediaserver:8096" disabled`) {
		t.Fatalf("expected env-managed media server URL to render disabled with env value, got: %s", body)
	}
	if strings.Contains(body, `value="http://stored-mediaserver:8096"`) {
		t.Fatal("expected env-managed field not to show stale stored value")
	}
}

func TestAdminSettingsPostUpdatesStoredSetupConfigAndRestarts(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	if err := config.SaveSetup(context.Background(), db, security.NewCrypto(h.cfg.EncryptionKey), config.SetupInput{
		AppBaseURL:           "https://veyra.setup",
		MediaServerType:      config.MediaServerJellyfin.String(),
		MediaServerURL:       "http://mediaserver:8096",
		MediaServerPublicURL: "https://mediaserver.setup",
		MediaServerAPIKey:    "jf-secret",
	}); err != nil {
		t.Fatal(err)
	}
	restarted := make(chan struct{}, 1)
	h.restart = func() { restarted <- struct{}{} }

	get := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	h.AdminSettingsGet(get, getReq)
	cookies := get.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie")
	}

	form := url.Values{}
	form.Set("csrf_token", cookies[0].Value)
	form.Set("app_name", "Veyra")
	form.Set("accent_color", "#d43f24")
	form.Set("media_server_public_url", "https://jf.public")
	form.Set("seerr_public_url", "https://se.public")
	form.Set("show_recent_media", "on")
	form.Set("show_recent_requests", "on")
	form.Set("show_request_quota", "on")
	form.Set("show_download_queue", "on")
	form.Set("show_upcoming_calendar", "on")
	form.Set("setup_app_base_url", "https://veyra.setup")
	form.Set("setup_media_server_type", config.MediaServerJellyfin.String())
	form.Set("setup_media_server_url", "http://mediaserver-new:8096")
	form.Set("setup_media_server_public_url", "https://mediaserver.setup")

	post := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(cookies[0])
	h.AdminSettingsPost(post, postReq)
	if post.Code != http.StatusFound {
		t.Fatalf("want redirect got %d: %s", post.Code, post.Body.String())
	}
	if loc := post.Header().Get("Location"); loc != "/admin/settings?saved=restart" {
		t.Fatalf("expected restart redirect, got %q", loc)
	}
	select {
	case <-restarted:
	case <-time.After(time.Second):
		t.Fatal("expected setup setting update to request restart")
	}

	stored, _, err := config.StoredSetup(context.Background(), db, security.NewCrypto(h.cfg.EncryptionKey))
	if err != nil {
		t.Fatal(err)
	}
	if stored.MediaServerURL != "http://mediaserver-new:8096" {
		t.Fatalf("expected updated Jellyfin URL, got %q", stored.MediaServerURL)
	}
	if stored.MediaServerAPIKey != "jf-secret" {
		t.Fatal("expected blank setup API key field to preserve existing secret")
	}
}

func TestAdminSettingsPostRollsBackRegularSettingsWhenSetupSaveFails(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	if err := config.SaveSetup(context.Background(), db, security.NewCrypto(h.cfg.EncryptionKey), config.SetupInput{
		AppBaseURL:      "https://veyra.setup",
		MediaServerType: config.MediaServerJellyfin.String(),
		MediaServerURL:  "http://mediaserver:8096",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_setup_settings BEFORE UPDATE ON settings
WHEN NEW.key LIKE 'setup.%' BEGIN SELECT RAISE(ABORT, 'setup write rejected'); END`); err != nil {
		t.Fatal(err)
	}

	get := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	h.AdminSettingsGet(get, getReq)
	cookies := get.Result().Cookies()
	form := url.Values{}
	form.Set("csrf_token", cookies[0].Value)
	form.Set("app_name", "should-rollback")
	form.Set("accent_color", "#d43f24")
	form.Set("media_server_public_url", "https://jf.public")
	form.Set("seerr_public_url", "https://se.public")
	form.Set("setup_app_base_url", "https://veyra.changed")
	form.Set("setup_media_server_type", config.MediaServerJellyfin.String())
	form.Set("setup_media_server_url", "http://mediaserver:8096")
	post := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(cookies[0])
	h.AdminSettingsPost(post, postReq)
	if post.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", post.Code)
	}
	value, err := store.GetSetting(context.Background(), db, settingAppName)
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("regular setting committed despite setup failure: %q", value)
	}
}

func TestSeerrSearchReturnsResults(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{searchResults: []sr.SearchResult{{ID: 11, MediaType: "movie", Title: "Arrival", Year: "2016", Status: "Requestable", CanRequest: true}}}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/seerr/search?q=arrival", nil)
	h.SeerrSearch(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"title":"Arrival"`) || !strings.Contains(body, `"canRequest":true`) {
		t.Fatalf("unexpected search response: %s", body)
	}
}

func TestSeerrRequestPostCreatesRequestForResolvedUser(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	seerrFake := &fakeSeerr{resolvedUser: &sr.UserIdentity{ID: 7, Username: "u"}, createdRequest: sr.CreatedRequest{ID: 55, Status: "Pending"}}
	h.seerr = seerrFake

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-request-user", Username: "u", DisplayName: "U", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	form.Set("csrf_token", "csrf")
	form.Set("media_id", "123")
	form.Set("media_type", "tv")
	form.Set("seasons", "2,3")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/seerr/request", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "csrf"})

	wrapped := middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.SeerrRequestPost))
	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body %s", w.Code, w.Body.String())
	}
	if seerrFake.lastRequest.MediaID != 123 || seerrFake.lastRequest.UserID != 7 || seerrFake.lastRequest.MediaType != "tv" || len(seerrFake.lastRequest.Seasons) != 2 || seerrFake.lastRequest.Seasons[0] != 2 || seerrFake.lastRequest.Seasons[1] != 3 {
		t.Fatalf("unexpected Seerr request input: %+v", seerrFake.lastRequest)
	}
	if seerrFake.createCalls != 1 {
		t.Fatal("expected Seerr request call")
	}
}

func TestSeerrRequestPostRequiresExplicitTVSeason(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seasons string
	}{
		{name: "missing"},
		{name: "all", seasons: "all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, db := mkHandlers(t)
			defer db.Close()
			seerrFake := &fakeSeerr{resolvedUser: &sr.UserIdentity{ID: 7, Username: "u"}, createdRequest: sr.CreatedRequest{ID: 55, Status: "Pending"}}
			h.seerr = seerrFake

			urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-tv-season-" + tc.name, Username: "u", DisplayName: "U", IsAdmin: false})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", "127.0.0.1", "test-agent")
			if err != nil {
				t.Fatal(err)
			}
			form := url.Values{}
			form.Set("csrf_token", "csrf")
			form.Set("media_id", "123")
			form.Set("media_type", "tv")
			if tc.seasons != "" {
				form.Set("seasons", tc.seasons)
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/api/seerr/request", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
			r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "csrf"})

			wrapped := middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.SeerrRequestPost))
			wrapped.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400 got %d body %s", w.Code, w.Body.String())
			}
			if seerrFake.createCalls != 0 {
				t.Fatal("did not expect Seerr request call")
			}
		})
	}
}

func TestSeerrRequestPostRequiresResolvedUser(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{}

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-missing-seerr", Username: "u", DisplayName: "U", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	form.Set("csrf_token", "csrf")
	form.Set("media_id", "123")
	form.Set("media_type", "movie")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/seerr/request", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "csrf"})

	wrapped := middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.SeerrRequestPost))
	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d body %s", w.Code, w.Body.String())
	}
}

func TestSeerrRequestPostReturnsSanitizedUpstreamError(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{
		resolvedUser: &sr.UserIdentity{ID: 7, Username: "u"},
		createErr:    errors.New("seerr request failed: 500 token abc123 leaked by upstream"),
	}

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-seerr-error", Username: "u", DisplayName: "U", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	form.Set("csrf_token", "csrf")
	form.Set("media_id", "123")
	form.Set("media_type", "movie")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/seerr/request", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "csrf"})

	wrapped := middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.SeerrRequestPost))
	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502 got %d body %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Seerr could not create that request.") {
		t.Fatalf("expected generic error, got %s", body)
	}
	if strings.Contains(strings.ToLower(body), "token") || strings.Contains(body, "abc123") {
		t.Fatalf("response leaked upstream detail: %s", body)
	}
}

func TestHomeRedirectsToDashboardWithValidSession(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{
		MediaServerUserID: "jf-home-user",
		Username:          "user",
		DisplayName:       "User",
		IsAdmin:           false,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(
		context.Background(),
		auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: urow.IsAdmin},
		"tok",
		"127.0.0.1",
		"test-agent",
	)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	h.Home(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302 got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/dashboard" {
		t.Fatalf("want /dashboard got %q", got)
	}
}

func TestHomeRedirectsToAdminWithValidAdminSession(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{
		MediaServerUserID: "jf-home-admin",
		Username:          "admin",
		DisplayName:       "Admin",
		IsAdmin:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(
		context.Background(),
		auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: urow.IsAdmin},
		"tok",
		"127.0.0.1",
		"test-agent",
	)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	h.Home(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302 got %d", w.Code)
	}
	if got := w.Result().Header.Get("Location"); got != "/admin" {
		t.Fatalf("want /admin got %q", got)
	}
}

type fakeSeerr struct {
	searchResults  []sr.SearchResult
	resolvedUser   *sr.UserIdentity
	createdRequest sr.CreatedRequest
	createErr      error
	lastRequest    sr.CreateRequestInput
	createCalls    int
}

func (f *fakeSeerr) ID() string   { return "seerr" }
func (f *fakeSeerr) Name() string { return "Seerr" }
func (f *fakeSeerr) Kind() string { return "requests" }

func (f *fakeSeerr) Health(context.Context) integrations.HealthStatus {
	return integrations.HealthStatus{OK: true, Message: "Online"}
}

func (f *fakeSeerr) Search(context.Context, string, int) ([]sr.SearchResult, error) {
	return f.searchResults, nil
}

func (f *fakeSeerr) CreateRequest(_ context.Context, in sr.CreateRequestInput) (sr.CreatedRequest, error) {
	f.lastRequest = in
	f.createCalls++
	if f.createErr != nil {
		return sr.CreatedRequest{}, f.createErr
	}
	return f.createdRequest, nil
}

func (f *fakeSeerr) RecentRequestsForUser(context.Context, sr.UserIdentity, int) ([]dashboard.RequestItem, error) {
	return nil, nil
}

func (f *fakeSeerr) UserQuotaForUser(context.Context, sr.UserIdentity) (*sr.Quota, error) {
	return nil, nil
}

func (f *fakeSeerr) ResolveUser(context.Context, string, string) (*sr.UserIdentity, error) {
	return f.resolvedUser, nil
}

func (f *fakeSeerr) ResolveUserByMediaServerID(context.Context, string) (*sr.UserIdentity, error) {
	return f.resolvedUser, nil
}
