package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/security"
)

func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	settings := readSettings(r, h.db,
		settingAppName,
		settingSeerrPublicURL,
		settingMediaServerPublicURL,
		settingWidgetRequestBot,
		settingWidgetRecentMedia,
		settingWidgetRecentRequests,
		settingWidgetRequestQuota,
		settingWidgetDownloadQueue,
		settingWidgetCalendar,
		settingDashboardMessage,
	)
	view := ViewData{
		AppName:                 appNameFromSettings(settings, h.cfg.AppName),
		CSRFToken:               middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
		Now:                     time.Now(),
		User:                    u,
		MediaServerName:         h.cfg.MediaServerType.Label(),
		SeerrPublicURL:          withDefault(readSettingFromMap(settings, settingSeerrPublicURL), h.cfg.SeerrPublicURL),
		MediaServerPublicURL:    withDefault(readSettingFromMap(settings, settingMediaServerPublicURL), h.cfg.MediaServerPublicURL),
		QuotaNote:               "Open Seerr to view request limits.",
		SettingsShowRequestBot:  readBoolSettingFromMap(settings, settingWidgetRequestBot, true),
		SettingsShowRecentMedia: readBoolSettingFromMap(settings, settingWidgetRecentMedia, true),
		SettingsShowRecentReqs:  readBoolSettingFromMap(settings, settingWidgetRecentRequests, true),
		SettingsShowQuota:       readBoolSettingFromMap(settings, settingWidgetRequestQuota, true),
		SettingsShowQueue:       readBoolSettingFromMap(settings, settingWidgetDownloadQueue, true),
		SettingsShowCalendar:    readBoolSettingFromMap(settings, settingWidgetCalendar, true),
		DashboardMessage:        strings.TrimSpace(readSettingFromMap(settings, settingDashboardMessage)),
	}

	if sess, ok := middleware.SessionFromContext(r.Context()); ok {
		view.SessionCreatedAt = formatSessionTime(sess.CreatedAt)
		view.SessionLastSeenAt = formatSessionTime(sess.LastSeenAt)
		view.SessionExpiresAt = formatSessionTime(sess.ExpiresAt)
		view.SessionIP = sess.IPAddress
		view.SessionUserAgent = sess.UserAgent
	}

	seerrConfigured := strings.TrimSpace(h.cfg.SeerrURL) != "" && strings.TrimSpace(h.cfg.SeerrAPIKey) != ""
	view.SettingsShowRequestBot = view.SettingsShowRequestBot && seerrConfigured
	view.SettingsShowRecentReqs = view.SettingsShowRecentReqs && seerrConfigured
	view.SettingsShowQuota = view.SettingsShowQuota && seerrConfigured
	arrConfigured := h.sonarrConfigured() || h.radarrConfigured()
	view.SettingsShowQueue = view.SettingsShowQueue && arrConfigured
	view.SettingsShowCalendar = view.SettingsShowCalendar && arrConfigured
	var seerrUser seerr.UserIdentity
	var identityErr error
	if view.SettingsShowRequestBot || view.SettingsShowRecentReqs || view.SettingsShowQuota {
		seerrUser, identityErr = h.resolveSeerrUser(r, u)
		if identityErr != nil {
			view.RequestAccessNote = "Request details are unavailable right now. Try again shortly."
			if errors.Is(identityErr, seerr.ErrUserNotLinked) {
				view.RequestAccessNote = "Link your " + view.MediaServerName + " account in Seerr to request media and see your requests."
			}
		}
	}

	var (
		mediaStat integrations.HealthStatus
		sStat     integrations.HealthStatus

		recentlyAdded          []dashboard.MediaItem
		recentMediaUnavailable bool

		recentRequests     []dashboard.RequestItem
		recentRequestsNote = "No requests yet."

		quota     quotaDisplay
		quotaNote = view.QuotaNote

		downloadQueue []dashboard.QueueItem
		queueNote     string
		calendarNote  string

		calendarWeekOffset int
		calendarWeekLabel  string
		calendarPrevURL    string
		calendarNextURL    string
		upcomingCalendar   []dashboard.CalendarItem
		calendarWeekStart  time.Time
		staleData          atomic.Bool
	)

	loaders := []func(){
		func() {
			mediaStat = h.cachedHealth(r, "health:media_server", h.mediaserver)
		},
		func() {
			sStat = h.cachedHealth(r, "health:seerr", h.seerr)
		},
	}

	if view.SettingsShowRecentMedia {
		loaders = append(loaders, func() {
			if !h.mediaserver.HasAPIKey() {
				recentMediaUnavailable = true
				return
			}
			cacheKey := "media:recent:" + strings.ToLower(strings.TrimSpace(u.MediaServerUserID))
			items, stale, err := cacheLoadJSONWithStale(h, r.Context(), cacheKey, cacheTTLWidget, cacheMaxStaleWidget, func() ([]dashboard.MediaItem, error) {
				return h.fetchMediaRecentlyAddedWithAPIKey(r.Context(), u.MediaServerUserID)
			})
			if err == nil {
				recentlyAdded = items
				if stale {
					staleData.Store(true)
				}
			} else {
				recentMediaUnavailable = true
			}
		})
	}

	if view.SettingsShowRecentReqs && identityErr != nil {
		recentRequestsNote = view.RequestAccessNote
	} else if view.SettingsShowRecentReqs {
		loaders = append(loaders, func() {
			cacheKey := fmt.Sprintf("requests:recent:user:%d", seerrUser.ID)
			if reqs, stale, err := cacheLoadJSONWithStale(h, r.Context(), cacheKey, cacheTTLWidget, cacheMaxStaleWidget, func() ([]dashboard.RequestItem, error) {
				return h.seerr.RecentRequestsForUser(r.Context(), seerrUser, 5)
			}); err == nil {
				recentRequests = reqs
				if stale {
					staleData.Store(true)
				}
			} else {
				recentRequestsNote = "Recent requests are unavailable right now. Try again shortly."
			}
		})
	}

	if view.SettingsShowQuota && identityErr != nil {
		quotaNote = view.RequestAccessNote
	} else if view.SettingsShowQuota {
		loaders = append(loaders, func() {
			cacheKey := fmt.Sprintf("requests:quota:user:%d", seerrUser.ID)
			q, stale, err := cacheLoadJSONWithStale(h, r.Context(), cacheKey, cacheTTLWidget, cacheMaxStaleWidget, func() (seerr.Quota, error) {
				qq, err := h.seerr.UserQuotaForUser(r.Context(), seerrUser)
				if err != nil {
					return seerr.Quota{}, err
				}
				if qq == nil {
					return seerr.Quota{}, fmt.Errorf("quota unavailable")
				}
				return *qq, nil
			})
			if err == nil {
				quota = quotaViewValues(q)
				quotaNote = ""
				if stale {
					staleData.Store(true)
				}
				return
			}
			quotaNote = "Request limits are unavailable right now. Try again shortly."
		})
	} else {
		quotaNote = ""
	}

	if view.SettingsShowQueue {
		loaders = append(loaders, func() {
			cacheKey := "downloads:queue"
			var stale bool
			var err error
			downloadQueue, stale, err = cacheLoadJSONWithStale(h, r.Context(), cacheKey, cacheTTLHealth, cacheMaxStaleWidget, func() ([]dashboard.QueueItem, error) {
				return h.combinedDownloadQueue(r.Context(), 0)
			})
			if err != nil {
				queueNote = providerUnavailableNote("Download queue", err)
			}
			if stale {
				staleData.Store(true)
			}
		})
	}

	if view.SettingsShowCalendar {
		nowUTC := time.Now().UTC()
		weekOffset := clampCalendarWeekOffset(parseCalendarWeekOffset(r), -12, 24)
		weekStart := weekStartMondayUTC(nowUTC).AddDate(0, 0, weekOffset*7)
		weekEnd := weekStart.AddDate(0, 0, 7)
		calendarWeekOffset = weekOffset
		calendarWeekLabel = fmt.Sprintf("%s - %s", weekStart.Format("Jan 02"), weekEnd.AddDate(0, 0, -1).Format("Jan 02"))
		calendarPrevURL = fmt.Sprintf("/dashboard?week=%d", weekOffset-1)
		calendarNextURL = fmt.Sprintf("/dashboard?week=%d", weekOffset+1)
		calendarWeekStart = weekStart

		loaders = append(loaders, func() {
			var stale bool
			var err error
			upcomingCalendar, stale, err = h.loadUpcomingCalendarWeek(r.Context(), weekStart, weekEnd)
			if err != nil {
				calendarNote = providerUnavailableNote("Calendar", err)
			}
			if stale {
				staleData.Store(true)
			}
		})
	}

	runConcurrently(loaders...)

	view.MediaServerStatus = boolToStatus(mediaStat.OK)
	view.SeerrStatus = boolToStatus(sStat.OK)
	view.DashboardServices = dashboardServices(
		view.MediaServerName,
		view.MediaServerPublicURL,
		view.MediaServerStatus,
		view.SeerrPublicURL,
		view.SeerrStatus,
	)
	view.RecentlyAdded = recentlyAdded
	view.DashboardDataStale = staleData.Load()
	view.RecentMediaUnavailable = recentMediaUnavailable
	view.RecentRequests = recentRequests
	view.RecentRequestsNote = recentRequestsNote
	view.QuotaValue = quota.Value
	view.QuotaMeters = quota.Meters
	view.QuotaNote = quotaNote
	view.DownloadQueue = downloadQueue
	view.QueueNote = queueNote
	view.CalendarNote = calendarNote
	view.CalendarWeekOffset = calendarWeekOffset
	view.CalendarWeekLabel = calendarWeekLabel
	view.CalendarPrevURL = calendarPrevURL
	view.CalendarNextURL = calendarNextURL
	view.UpcomingCalendar = upcomingCalendar
	if view.SettingsShowCalendar && (calendarNote == "" || len(upcomingCalendar) > 0) {
		view.UpcomingCalendarGroups = groupCalendarItems(view.UpcomingCalendar, calendarWeekStart)
	}

	h.render(w, "dashboard.html", view)
}

func runConcurrently(tasks ...func()) {
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		panicValue any
	)
	wg.Add(len(tasks))
	for _, task := range tasks {
		task := task
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					mu.Lock()
					if panicValue == nil {
						panicValue = recovered
					}
					mu.Unlock()
				}
			}()
			task()
		}()
	}
	wg.Wait()
	if panicValue != nil {
		panic(panicValue)
	}
}

func (h *Handlers) combinedDownloadQueue(ctx context.Context, limit int) ([]dashboard.QueueItem, error) {
	var clients []downloadQueueClient
	if h.sonarrConfigured() {
		clients = append(clients, h.sonarr)
	}
	if h.radarrConfigured() {
		clients = append(clients, h.radarr)
	}
	return combineDownloadQueue(ctx, h.debug, limit, clients...)
}

type downloadQueueClient interface {
	Name() string
	Queue(ctx context.Context, limit int) ([]dashboard.QueueItem, error)
}

func combineDownloadQueue(ctx context.Context, debug func(string, ...any), limit int, clients ...downloadQueueClient) ([]dashboard.QueueItem, error) {
	loads := make([]providerLoad[dashboard.QueueItem], 0, len(clients))
	for _, client := range clients {
		if client == nil {
			continue
		}
		client := client
		loads = append(loads, providerLoad[dashboard.QueueItem]{name: client.Name(), load: func() ([]dashboard.QueueItem, error) {
			return client.Queue(ctx, limit)
		}})
	}
	out, err := collectProviderItems(loads, func(name string, err error) {
		debug("download queue unavailable", "service", name, "err", security.RedactErr(err))
	})
	sortDownloadQueueItems(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	debug("download queue combined", "total", len(out), "limit", limit)
	return out, err
}

func sortDownloadQueueItems(out []dashboard.QueueItem) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Progress != out[j].Progress {
			return out[i].Progress > out[j].Progress
		}
		if out[i].SortTime.IsZero() && out[j].SortTime.IsZero() {
			if out[i].Source == out[j].Source {
				return out[i].SortIndex < out[j].SortIndex
			}
			return out[i].Source < out[j].Source
		}
		if out[i].SortTime.IsZero() {
			return false
		}
		if out[j].SortTime.IsZero() {
			return true
		}
		return out[i].SortTime.Before(out[j].SortTime)
	})
}

func (h *Handlers) combinedUpcomingCalendar(ctx context.Context, start, end time.Time, limit int) ([]dashboard.CalendarItem, error) {
	var clients []upcomingCalendarClient
	if h.sonarrConfigured() {
		clients = append(clients, h.sonarr)
	}
	if h.radarrConfigured() {
		clients = append(clients, h.radarr)
	}
	return combineUpcomingCalendar(ctx, h.debug, start, end, limit, clients...)
}

type upcomingCalendarClient interface {
	Name() string
	UpcomingWindow(ctx context.Context, start, end time.Time, limit int) ([]dashboard.CalendarItem, error)
}

func combineUpcomingCalendar(ctx context.Context, debug func(string, ...any), start, end time.Time, limit int, clients ...upcomingCalendarClient) ([]dashboard.CalendarItem, error) {
	loads := make([]providerLoad[dashboard.CalendarItem], 0, len(clients))
	for _, client := range clients {
		if client == nil {
			continue
		}
		client := client
		loads = append(loads, providerLoad[dashboard.CalendarItem]{name: client.Name(), load: func() ([]dashboard.CalendarItem, error) {
			return client.UpcomingWindow(ctx, start, end, limit)
		}})
	}
	out, err := collectProviderItems(loads, func(name string, err error) {
		debug("calendar unavailable", "service", name, "err", security.RedactErr(err))
	})
	sortCalendarItems(out)
	return limitCalendarItems(out, limit), err
}

type providerLoad[T any] struct {
	name string
	load func() ([]T, error)
}

func collectProviderItems[T any](loads []providerLoad[T], onFailure func(string, error)) ([]T, error) {
	if len(loads) == 0 {
		return []T{}, nil
	}
	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		out       []T
		succeeded int
		failures  []error
	)
	wg.Add(len(loads))
	for _, provider := range loads {
		provider := provider
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					err := fmt.Errorf("provider %s panicked: %v", provider.name, recovered)
					mu.Lock()
					failures = append(failures, err)
					mu.Unlock()
					if onFailure != nil {
						onFailure(provider.name, err)
					}
				}
			}()
			items, err := provider.load()
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
				if onFailure != nil {
					onFailure(provider.name, err)
				}
				return
			}
			mu.Lock()
			succeeded++
			out = append(out, items...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(failures) > 0 {
		if succeeded > 0 {
			return out, &partialProviderError{errors.Join(failures...)}
		}
		return nil, errors.Join(failures...)
	}
	return out, nil
}

type partialProviderError struct{ error }

func providerUnavailableNote(widget string, err error) string {
	var partial *partialProviderError
	if errors.As(err, &partial) {
		return widget + " is incomplete because a service is unavailable. Try again shortly."
	}
	return widget + " is unavailable right now. Try again shortly."
}

func (h *Handlers) loadUpcomingCalendarWeek(ctx context.Context, start, end time.Time) ([]dashboard.CalendarItem, bool, error) {
	cacheKey := "calendar:upcoming:" + start.Format("2006-01-02")
	items, stale, err := cacheLoadJSONWithStale(h, ctx, cacheKey, cacheTTLCalendar, cacheMaxStaleWidget, func() ([]dashboard.CalendarItem, error) {
		return h.combinedUpcomingCalendar(ctx, start, end, 0)
	})
	return items, stale, err
}

func sortCalendarItems(items []dashboard.CalendarItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].AirsAt.IsZero() && items[j].AirsAt.IsZero() {
			return items[i].Title < items[j].Title
		}
		if items[i].AirsAt.IsZero() {
			return false
		}
		if items[j].AirsAt.IsZero() {
			return true
		}
		return items[i].AirsAt.Before(items[j].AirsAt)
	})
}

func limitCalendarItems(items []dashboard.CalendarItem, limit int) []dashboard.CalendarItem {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func groupCalendarItems(items []dashboard.CalendarItem, weekStart time.Time) []CalendarGroup {
	base := weekStartMondayUTC(weekStart)
	out := make([]CalendarGroup, 0, 7)
	byDay := make(map[time.Time][]dashboard.CalendarItem, 7)
	today := time.Now().UTC()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	for _, item := range items {
		if item.AirsAt.IsZero() {
			continue
		}
		airsAt := item.AirsAt.UTC()
		dayDate := time.Date(airsAt.Year(), airsAt.Month(), airsAt.Day(), 0, 0, 0, 0, time.UTC)
		if dayDate.Before(base) || !dayDate.Before(base.AddDate(0, 0, 7)) {
			continue
		}
		byDay[dayDate] = append(byDay[dayDate], item)
	}
	for i := 0; i < 7; i++ {
		day := base.AddDate(0, 0, i)
		label := day.Format("Mon, Jan 02")
		dayItems := byDay[day]
		if len(dayItems) > 1 {
			sortCalendarItems(dayItems)
		}
		out = append(out, CalendarGroup{Label: label, Date: day.Format("2006-01-02"), Items: dayItems, Count: len(dayItems), IsToday: day.Equal(todayDate)})
	}
	return out
}

func weekStartMondayUTC(now time.Time) time.Time {
	dayStart := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(dayStart.Weekday()) + 6) % 7
	return dayStart.AddDate(0, 0, -offset)
}

func parseCalendarWeekOffset(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("week"))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func clampCalendarWeekOffset(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func (h *Handlers) resolveSeerrUser(r *http.Request, u auth.User) (seerr.UserIdentity, error) {
	mediaID := strings.TrimSpace(u.MediaServerUserID)
	if mediaID == "" {
		return seerr.UserIdentity{}, seerr.ErrUserNotLinked
	}
	return cacheLoadJSON(h, r.Context(), "seerr:user:media_server:"+mediaID, cacheTTLResolvedUser, func() (seerr.UserIdentity, error) {
		resolved, err := h.seerr.ResolveUserByMediaServerID(r.Context(), mediaID)
		if err != nil {
			return seerr.UserIdentity{}, err
		}
		if resolved == nil || resolved.ID <= 0 {
			return seerr.UserIdentity{}, seerr.ErrUserNotLinked
		}
		return *resolved, nil
	})
}

func (h *Handlers) sonarrConfigured() bool {
	return strings.TrimSpace(h.cfg.SonarrURL) != "" && strings.TrimSpace(h.cfg.SonarrAPIKey) != ""
}
func (h *Handlers) radarrConfigured() bool {
	return strings.TrimSpace(h.cfg.RadarrURL) != "" && strings.TrimSpace(h.cfg.RadarrAPIKey) != ""
}
