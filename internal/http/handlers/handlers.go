package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	veyra "github.com/mayvqt/veyra"
	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/integrations/arr"
	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
	"github.com/mayvqt/veyra/internal/integrations/seerr"
)

type Handlers struct {
	cfg          config.Config
	log          *slog.Logger
	db           *sql.DB
	tmpl         *template.Template
	authSvc      *auth.Service
	mediaserver  *mediaserver.Client
	seerr        seerrIntegration
	sonarr       *arr.Client
	radarr       *arr.Client
	prowlarr     *arr.Client
	restart      func()
	loginLimiter interface {
		Allow(ip, username string) bool
	}
	cacheNamespace string
	cacheMu        sync.Mutex
	cacheLoads     map[string]*cacheLoad
}

type seerrIntegration interface {
	integrations.Integration
	Search(ctx context.Context, query string, limit int) ([]seerr.SearchResult, error)
	CreateRequest(ctx context.Context, in seerr.CreateRequestInput) (seerr.CreatedRequest, error)
	RecentRequestsForUser(ctx context.Context, user seerr.UserIdentity, limit int) ([]dashboard.RequestItem, error)
	UserQuotaForUser(ctx context.Context, user seerr.UserIdentity) (*seerr.Quota, error)
	ResolveUserByMediaServerID(ctx context.Context, mediaserverUserID string) (*seerr.UserIdentity, error)
}

func New(cfg config.Config, log *slog.Logger, db *sql.DB, tmpl *template.Template, authSvc *auth.Service, mediaserver *mediaserver.Client, seerr *seerr.Client, sonarr *arr.Client, radarr *arr.Client, prowlarr *arr.Client, loginLimiter interface {
	Allow(ip, username string) bool
}) *Handlers {
	return &Handlers{cfg: cfg, log: log, db: db, tmpl: tmpl, authSvc: authSvc, mediaserver: mediaserver, seerr: seerr, sonarr: sonarr, radarr: radarr, prowlarr: prowlarr, restart: defaultRestart, loginLimiter: loginLimiter, cacheNamespace: integrationCacheNamespace(cfg)}
}

type ServiceStatus struct {
	Name         string
	Internal     string
	Public       string
	Health       string
	Configured   bool
	Required     bool
	LastError    string
	RecentErrors []string
	Version      string
	Runtime      string
	Stats        []AdminStat
	Issues       []string
	DiskWarnings []string
}

// AdminTimestamp keeps the machine-readable instant beside the compact label
// shown in admin screens.
type AdminTimestamp struct {
	Datetime string
	Label    string
}

type AdminStat struct {
	Label string
	Value string
}

type AdminPlaybackSession struct {
	User       string
	Title      string
	Client     string
	DeviceName string
	MediaType  string
}

type AdminActionItem struct {
	Title    string
	Detail   string
	Href     string
	Severity string
}

type AdminOverviewSummary struct {
	OnlineServices      int
	TotalServices       int
	ConfiguredServices  int
	TotalConnectors     int
	AdminShare          string
	LatestActivityAt    string
	HealthSummary       string
	ConfigurationStatus string
}

type AdminStatusRow struct {
	Label    string
	Value    string
	Severity string
}

type AdminUserView struct {
	Username          string
	DisplayName       string
	MediaServerUserID string
	MediaServerURL    string
	IsAdmin           bool
	CreatedAtTime     AdminTimestamp
	LastLoginTime     AdminTimestamp
}

type AuditLogView struct {
	Action        string
	Target        string
	Actor         string
	Message       string
	Detail        string
	Severity      string
	Category      string
	IPAddress     string
	UserID        string
	CreatedAt     string
	CreatedAtTime AdminTimestamp
	Metadata      string
}

type auditStats struct {
	RecentLoginCount int
	WarningLogCount  int
}

type CalendarGroup struct {
	Label   string
	Date    string
	Items   []dashboard.CalendarItem
	Count   int
	IsToday bool
}

type SetupConfigField struct {
	Section    string
	Label      string
	Name       string
	Value      string
	Secret     bool
	EnvManaged bool
	HasValue   bool
	Options    []SetupConfigOption
}

type SetupConfigOption struct {
	Value    string
	Label    string
	Selected bool
}

type SetupConfigGroup struct {
	Name   string
	Fields []SetupConfigField
}

type QuotaMeter struct {
	Label     string
	Value     string
	Detail    string
	Class     string
	Percent   int
	Unlimited bool
}

type DashboardService struct {
	Name   string
	URL    string
	Status string
}

type ViewData struct {
	AppName                   string
	CSRFToken                 string
	Now                       time.Time
	User                      auth.User
	Error                     string
	Success                   string
	MediaServerStatus         string
	MediaServerName           string
	SeerrStatus               string
	SonarrStatus              string
	RadarrStatus              string
	ProwlarrStatus            string
	RecentlyAdded             []dashboard.MediaItem
	RecentRequests            []dashboard.RequestItem
	RecentRequestsNote        string
	RequestAccessNote         string
	DownloadQueue             []dashboard.QueueItem
	QueueNote                 string
	CalendarNote              string
	UpcomingCalendar          []dashboard.CalendarItem
	UpcomingCalendarGroups    []CalendarGroup
	CalendarWeekOffset        int
	CalendarWeekLabel         string
	CalendarPrevURL           string
	CalendarNextURL           string
	QuotaNote                 string
	QuotaValue                string
	QuotaMeters               []QuotaMeter
	DashboardServices         []DashboardService
	SessionCreatedAt          string
	SessionLastSeenAt         string
	SessionExpiresAt          string
	SessionIP                 string
	SessionUserAgent          string
	SeerrPublicURL            string
	MediaServerPublicURL      string
	BrandAppName              string
	LogoURL                   string
	AccentStyle               template.CSS
	BrandLogoURL              string
	BrandAccent               string
	SettingsShowRequestBot    bool
	SettingsShowRecentMedia   bool
	RecentMediaUnavailable    bool
	SettingsShowRecentReqs    bool
	SettingsShowQuota         bool
	SettingsShowQueue         bool
	SettingsShowCalendar      bool
	SettingsMediaServerPublic string
	SettingsSeerrPublic       string
	ServiceStatuses           []ServiceStatus
	HasUnconfiguredServices   bool
	Playback                  []AdminPlaybackSession
	PlaybackUnavailable       bool
	Users                     []AdminUserView
	AuditLogs                 []AuditLogView
	AppVersion                string
	StaticVersion             string
	DatabaseStatus            string
	ConfigWarnings            []string
	DashboardMessage          string
	DashboardDataStale        bool
	TotalUsers                int
	AdminUsers                int
	StandardUsers             int
	RecentLoginCount          int
	WarningLogCount           int
	AdminActionItems          []AdminActionItem
	AdminOverviewSummary      AdminOverviewSummary
	AdminStatusRows           []AdminStatusRow
	SetupConfigGroups         []SetupConfigGroup
	SetupConfigFields         []SetupConfigField
	Setup                     config.SetupInput
	SetupComplete             bool
}

const (
	dashboardRecentMediaLimit = 12
	cacheTTLHealth            = 30 * time.Second
	cacheTTLWidget            = 45 * time.Second
	cacheTTLCalendar          = 2 * time.Minute
	cacheTTLResolvedUser      = 10 * time.Minute
	cacheMaxStaleWidget       = 15 * time.Minute
)

func (h *Handlers) render(w http.ResponseWriter, name string, data ViewData) {
	h.applyBranding(w, &data)
	if data.StaticVersion == "" {
		data.StaticVersion = veyra.StaticVersion()
	}
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		h.log.Error("template render failed", "template", name, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(buf.Bytes())
}

func defaultRestart() {
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
}

func (h *Handlers) debug(msg string, args ...any) {
	if h.log != nil {
		h.log.Debug(msg, args...)
	}
}

func (h *Handlers) warnPersistence(operation string, err error) {
	if err != nil && h.log != nil {
		h.log.Warn("persistence operation failed", "operation", operation, "err", err)
	}
}

func (h *Handlers) cachedHealth(r *http.Request, key string, client integrations.Integration) integrations.HealthStatus {
	status, _ := cacheLoadJSON(h, r.Context(), key, cacheTTLHealth, func() (integrations.HealthStatus, error) {
		return client.Health(r.Context()), nil
	})
	return status
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
