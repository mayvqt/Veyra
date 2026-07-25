package http

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/http/handlers"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations/arr"
	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
	"github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/security"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg config.Config, log *slog.Logger, db *sql.DB) (http.Handler, error) {
	tmpl, err := template.ParseGlob("internal/http/templates/*.html")
	if err != nil {
		return nil, err
	}

	crypto := security.NewCrypto(cfg.EncryptionKey)
	mediaserverClient, err := mediaserver.NewClient(cfg.MediaServerType, cfg.MediaServerURL, cfg.MediaServerPublicURL, cfg.MediaServerAPIKey)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(db, crypto, cfg.SessionSecret, mediaserverClient)
	seerrClient := seerr.NewClient(cfg.SeerrURL, cfg.SeerrPublicURL, cfg.SeerrAPIKey)
	sonarrClient := arr.NewClient("Sonarr", cfg.SonarrURL, cfg.SonarrAPIKey)
	radarrClient := arr.NewClient("Radarr", cfg.RadarrURL, cfg.RadarrAPIKey)
	prowlarrClient := arr.NewProwlarrClient(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
	loginLimiter := middleware.NewLoginRateLimiter(10, 10*time.Minute)
	h := handlers.New(cfg, log, db, tmpl, authSvc, mediaserverClient, seerrClient, sonarrClient, radarrClient, prowlarrClient, loginLimiter)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(middleware.TrustedProxy(cfg.TrustedProxyCIDRs))
	r.Use(middleware.RequestLog(log))
	r.Use(middleware.SecurityHeaders)

	r.Get("/healthz", h.Health)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	r.Get("/", h.Home)
	r.Get("/setup", h.SetupGet)
	r.Post("/setup", h.SetupPost)
	r.Get("/login", h.Login)
	r.Post("/login", h.LoginPost)
	r.Post("/logout", h.LogoutPost)

	r.Group(func(pr chi.Router) {
		pr.Use(middleware.RequireAuth(authSvc))
		pr.Get("/dashboard", h.Dashboard)
		pr.Get("/guide", h.Guide)
		pr.Get("/media/server/poster/{itemID}", h.MediaServerPoster)
		pr.Get("/api/seerr/search", h.SeerrSearch)
		pr.Post("/api/seerr/request", h.SeerrRequestPost)

		pr.Route("/admin", func(ar chi.Router) {
			ar.Use(middleware.RequireAdmin)
			ar.Get("/", h.Admin)
			ar.Get("/settings", h.AdminSettingsGet)
			ar.Post("/settings", h.AdminSettingsPost)
			ar.Get("/users", h.AdminUsers)
			ar.Get("/logs", h.AdminLogs)
			ar.Get("/playback", h.AdminPlayback)
			ar.Get("/integrations", h.AdminIntegrations)
		})
	})

	return r, nil
}
