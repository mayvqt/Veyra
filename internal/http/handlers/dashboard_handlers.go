package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/buildinfo"
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
		StaticVersion:           buildinfo.Version,
	}

	if sess, ok := middleware.SessionFromContext(r.Context()); ok {
		view.SessionCreatedAt = formatSessionTime(sess.CreatedAt)
		view.SessionLastSeenAt = formatSessionTime(sess.LastSeenAt)
		view.SessionExpiresAt = formatSessionTime(sess.ExpiresAt)
		view.SessionIP = sess.IPAddress
		view.SessionUserAgent = sess.UserAgent
	}

	var seerrUser seerr.UserIdentity
	needsSeerrUser := view.SettingsShowRecentReqs || view.SettingsShowQuota
	if needsSeerrUser {
		seerrUser = h.resolveSeerrUser(r, u)
	}

	var (
		mediaStat integrations.HealthStatus
		sStat     integrations.HealthStatus

		recentlyAdded          []dashboard.MediaItem
		recentMediaUnavailable bool

		recentRequests []dashboard.RequestItem

		quota     quotaDisplay
		quotaNote = view.QuotaNote

		downloadQueue []dashboard.QueueItem

		calendarWeekOffset int
		calendarWeekLabel  string
		calendarPrevURL    string
		calendarNextURL    string
		upcomingCalendar   []dashboard.CalendarItem
		calendarWeekStart  time.Time
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
			items, ok := h.fetchMediaRecentlyAddedWithAPIKey(r.Context(), u.MediaServerUserID)
			if ok {
				recentlyAdded = items
			} else {
				recentMediaUnavailable = true
			}
		})
	}

	if view.SettingsShowRecentReqs {
		loaders = append(loaders, func() {
			cacheKey := "requests:recent:v2:" + seerrUserCacheKey(seerrUser, u)
			if reqs, err := cacheLoadJSON(h, r.Context(), cacheKey, cacheTTLWidget, func() ([]dashboard.RequestItem, error) {
				return h.seerr.RecentRequestsForUser(r.Context(), seerrUser, 5)
			}); err == nil {
				recentRequests = reqs
			}
		})
	}

	if view.SettingsShowQuota {
		loaders = append(loaders, func() {
			cacheKey := "requests:quota:" + seerrUserCacheKey(seerrUser, u)
			q, err := cacheLoadJSON(h, r.Context(), cacheKey, cacheTTLWidget, func() (seerr.Quota, error) {
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
				return
			}
			if seerrUser.ID == 0 {
				quotaNote = "Link this " + view.MediaServerName + " user in Seerr to view request limits."
			}
		})
	} else {
		quotaNote = ""
	}

	if view.SettingsShowQueue {
		loaders = append(loaders, func() {
			cacheKey := "downloads:queue"
			downloadQueue, _ = cacheLoadJSON(h, r.Context(), cacheKey, cacheTTLHealth, func() ([]dashboard.QueueItem, error) {
				return h.combinedDownloadQueue(r.Context(), 0), nil
			})
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
			upcomingCalendar = h.loadUpcomingCalendarWeek(r.Context(), weekStart, weekEnd)
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
	view.RecentMediaUnavailable = recentMediaUnavailable
	view.RecentRequests = recentRequests
	view.QuotaValue = quota.Value
	view.QuotaMeters = quota.Meters
	view.QuotaNote = quotaNote
	view.DownloadQueue = downloadQueue
	view.CalendarWeekOffset = calendarWeekOffset
	view.CalendarWeekLabel = calendarWeekLabel
	view.CalendarPrevURL = calendarPrevURL
	view.CalendarNextURL = calendarNextURL
	view.UpcomingCalendar = upcomingCalendar
	if view.SettingsShowCalendar {
		view.UpcomingCalendarGroups = groupCalendarItems(view.UpcomingCalendar, calendarWeekStart)
	}

	h.render(w, "dashboard.html", view)
}

func runConcurrently(tasks ...func()) {
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go func() {
			defer wg.Done()
			task()
		}()
	}
	wg.Wait()
}

func (h *Handlers) combinedDownloadQueue(ctx context.Context, limit int) []dashboard.QueueItem {
	return combineDownloadQueue(ctx, h.debug, limit, h.sonarr, h.radarr)
}

type downloadQueueClient interface {
	Name() string
	Queue(ctx context.Context, limit int) ([]dashboard.QueueItem, error)
}

func combineDownloadQueue(ctx context.Context, debug func(string, ...any), limit int, clients ...downloadQueueClient) []dashboard.QueueItem {
	out := make([]dashboard.QueueItem, 0, max(limit, minCollectionCapacity))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, client := range clients {
		if client == nil {
			continue
		}
		wg.Add(1)
		go func(client downloadQueueClient) {
			defer wg.Done()
			items, err := client.Queue(ctx, limit)
			if err != nil {
				debug("download queue unavailable", "service", client.Name(), "err", security.RedactErr(err))
				return
			}
			debug("download queue fetched", "service", client.Name(), "count", len(items))
			mu.Lock()
			out = append(out, items...)
			mu.Unlock()
		}(client)
	}
	wg.Wait()
	sortDownloadQueueItems(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	debug("download queue combined", "total", len(out), "limit", limit)
	return out
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

func (h *Handlers) combinedUpcomingCalendar(ctx context.Context, start, end time.Time, limit int) []dashboard.CalendarItem {
	return combineUpcomingCalendar(ctx, h.debug, start, end, limit, h.sonarr, h.radarr)
}

type upcomingCalendarClient interface {
	Name() string
	UpcomingWindow(ctx context.Context, start, end time.Time, limit int) ([]dashboard.CalendarItem, error)
}

func combineUpcomingCalendar(ctx context.Context, debug func(string, ...any), start, end time.Time, limit int, clients ...upcomingCalendarClient) []dashboard.CalendarItem {
	out := make([]dashboard.CalendarItem, 0, max(limit, minCollectionCapacity))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, client := range clients {
		if client == nil {
			continue
		}
		wg.Add(1)
		go func(client upcomingCalendarClient) {
			defer wg.Done()
			items, err := client.UpcomingWindow(ctx, start, end, limit)
			if err != nil {
				debug("calendar unavailable", "service", client.Name(), "err", security.RedactErr(err))
				return
			}
			mu.Lock()
			out = append(out, items...)
			mu.Unlock()
		}(client)
	}
	wg.Wait()
	sortCalendarItems(out)
	return limitCalendarItems(out, limit)
}

func (h *Handlers) loadUpcomingCalendarWeek(ctx context.Context, start, end time.Time) []dashboard.CalendarItem {
	cacheKey := "calendar:upcoming:" + start.Format("2006-01-02")
	items, _ := cacheLoadJSON(h, ctx, cacheKey, cacheTTLCalendar, func() ([]dashboard.CalendarItem, error) {
		return h.combinedUpcomingCalendar(ctx, start, end, 0), nil
	})
	return items
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

func (h *Handlers) resolveSeerrUser(r *http.Request, u auth.User) seerr.UserIdentity {
	var cached seerr.UserIdentity
	if strings.TrimSpace(u.MediaServerUserID) != "" {
		cacheKey := "seerr:user:media_server:" + strings.ToLower(u.MediaServerUserID)
		if cacheGetJSON(r.Context(), h.db, cacheKey, &cached) && cached.ID > 0 {
			return cached
		}
		if resolved, err := h.seerr.ResolveUserByMediaServerID(r.Context(), u.MediaServerUserID); err == nil && resolved != nil && resolved.ID > 0 {
			h.cacheSetJSON(r.Context(), cacheKey, *resolved, cacheTTLResolvedUser)
			return *resolved
		}
	}
	fallbackCacheKey := "seerr:user:name:" + strings.ToLower(u.Username)
	if cacheGetJSON(r.Context(), h.db, fallbackCacheKey, &cached) && cached.ID > 0 {
		return cached
	}
	resolved, err := h.seerr.ResolveUser(r.Context(), u.Username, u.DisplayName)
	if err == nil && resolved != nil && resolved.ID > 0 {
		h.cacheSetJSON(r.Context(), fallbackCacheKey, *resolved, cacheTTLResolvedUser)
		return *resolved
	}
	return seerr.UserIdentity{Username: u.Username, DisplayName: u.DisplayName}
}

func seerrUserCacheKey(seerrUser seerr.UserIdentity, u auth.User) string {
	if seerrUser.ID > 0 {
		return fmt.Sprintf("%d", seerrUser.ID)
	}
	return strings.ToLower(u.Username)
}
