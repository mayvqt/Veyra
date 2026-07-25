package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/testutil"
)

func TestSortCalendarItems(t *testing.T) {
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	items := []dashboard.CalendarItem{
		{Title: "Later", AirsAt: base.Add(48 * time.Hour)},
		{Title: "No Date", AirsAt: time.Time{}},
		{Title: "Soon", AirsAt: base.Add(12 * time.Hour)},
	}
	sortCalendarItems(items)
	if items[0].Title != "Soon" || items[1].Title != "Later" || items[2].Title != "No Date" {
		t.Fatalf("unexpected sort order: %+v", items)
	}
}

func TestLimitCalendarItems(t *testing.T) {
	items := []dashboard.CalendarItem{{Title: "1"}, {Title: "2"}, {Title: "3"}}
	got := limitCalendarItems(items, 2)
	if len(got) != 2 || got[0].Title != "1" || got[1].Title != "2" {
		t.Fatalf("unexpected limit behavior: %+v", got)
	}
	if keep := limitCalendarItems(items, 0); len(keep) != 3 {
		t.Fatalf("expected all items for limit 0, got %d", len(keep))
	}
}

func TestSortDownloadQueueItemsPrioritizesProgress(t *testing.T) {
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	items := []dashboard.QueueItem{
		{Title: "Soonest", Progress: 20, SortTime: base.Add(time.Hour)},
		{Title: "Most Complete", Progress: 80, SortTime: base.Add(3 * time.Hour)},
		{Title: "Tie Earlier", Progress: 80, SortTime: base.Add(2 * time.Hour)},
	}

	sortDownloadQueueItems(items)

	if items[0].Title != "Tie Earlier" || items[1].Title != "Most Complete" || items[2].Title != "Soonest" {
		t.Fatalf("unexpected queue sort order: %+v", items)
	}
}

func TestGroupCalendarItems(t *testing.T) {
	now := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC) // Wednesday
	items := []dashboard.CalendarItem{
		{Title: "A", AirsAt: time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)},
		{Title: "B", AirsAt: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)},
		{Title: "C", AirsAt: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)},
		{Title: "D", AirsAt: time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC)}, // Sunday
		{Title: "Out Of Week", AirsAt: time.Date(2026, 5, 25, 11, 0, 0, 0, time.UTC)},
	}

	groups := groupCalendarItems(items, now)
	if len(groups) != 7 {
		t.Fatalf("expected 7 date groups, got %d", len(groups))
	}
	if groups[0].Label != "Mon, May 18" || len(groups[0].Items) != 0 {
		t.Fatalf("unexpected first group: %+v", groups[0])
	}
	if groups[2].Label != "Wed, May 20" || len(groups[2].Items) != 2 {
		t.Fatalf("unexpected Wednesday group: %+v", groups[2])
	}
	if groups[2].Items[0].Title != "B" || groups[2].Items[1].Title != "A" {
		t.Fatalf("expected Wednesday items sorted by time, got %+v", groups[2].Items)
	}
	if groups[6].Label != "Sun, May 24" || len(groups[6].Items) != 1 {
		t.Fatalf("unexpected Sunday group: %+v", groups[6])
	}
}

func TestCombinedDownloadQueueFetchesArrClientsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan []dashboard.QueueItem, 1)

	go func() {
		done <- combineDownloadQueue(
			context.Background(),
			func(string, ...any) {},
			10,
			blockingQueueClient{name: "Sonarr", started: started, release: release, items: []dashboard.QueueItem{{Title: "Andor", Source: "Sonarr", Progress: 75}}},
			blockingQueueClient{name: "Radarr", started: started, release: release, items: []dashboard.QueueItem{{Title: "Heat", Source: "Radarr", Progress: 50}}},
		)
	}()

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "Sonarr", "Radarr")
	close(release)
	items := <-done
	if len(items) != 2 {
		t.Fatalf("expected both queue results, got %+v", items)
	}
}

func TestCombinedUpcomingCalendarFetchesArrClientsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan []dashboard.CalendarItem, 1)

	go func() {
		done <- combineUpcomingCalendar(
			context.Background(),
			func(string, ...any) {},
			time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
			10,
			blockingCalendarClient{name: "Sonarr", started: started, release: release, items: []dashboard.CalendarItem{{Title: "Andor", Source: "Sonarr", AirsAt: time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)}}},
			blockingCalendarClient{name: "Radarr", started: started, release: release, items: []dashboard.CalendarItem{{Title: "Heat 2", Source: "Radarr", AirsAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)}}},
		)
	}()

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "Sonarr", "Radarr")
	close(release)
	items := <-done
	if len(items) != 2 {
		t.Fatalf("expected both calendar results, got %+v", items)
	}
}

type blockingQueueClient struct {
	name    string
	started chan<- string
	release <-chan struct{}
	items   []dashboard.QueueItem
}

func (c blockingQueueClient) Name() string { return c.name }

func (c blockingQueueClient) Queue(context.Context, int) ([]dashboard.QueueItem, error) {
	c.started <- c.name
	<-c.release
	return c.items, nil
}

type blockingCalendarClient struct {
	name    string
	started chan<- string
	release <-chan struct{}
	items   []dashboard.CalendarItem
}

func (c blockingCalendarClient) Name() string { return c.name }

func (c blockingCalendarClient) UpcomingWindow(context.Context, time.Time, time.Time, int) ([]dashboard.CalendarItem, error) {
	c.started <- c.name
	<-c.release
	return c.items, nil
}
