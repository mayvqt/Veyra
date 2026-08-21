package handlers

import (
	"context"
	"errors"
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
	type result struct {
		items []dashboard.QueueItem
		err   error
	}
	done := make(chan result, 1)

	go func() {
		items, err := combineDownloadQueue(
			context.Background(),
			func(string, ...any) {},
			10,
			blockingQueueClient{name: "Sonarr", started: started, release: release, items: []dashboard.QueueItem{{Title: "Andor", Source: "Sonarr", Progress: 75}}},
			blockingQueueClient{name: "Radarr", started: started, release: release, items: []dashboard.QueueItem{{Title: "Heat", Source: "Radarr", Progress: 50}}},
		)
		done <- result{items: items, err: err}
	}()

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "Sonarr", "Radarr")
	close(release)
	got := <-done
	if got.err != nil || len(got.items) != 2 {
		t.Fatalf("expected both queue results, got items=%+v err=%v", got.items, got.err)
	}
}

func TestCombinedUpcomingCalendarFetchesArrClientsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	type result struct {
		items []dashboard.CalendarItem
		err   error
	}
	done := make(chan result, 1)

	go func() {
		items, err := combineUpcomingCalendar(
			context.Background(),
			func(string, ...any) {},
			time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
			10,
			blockingCalendarClient{name: "Sonarr", started: started, release: release, items: []dashboard.CalendarItem{{Title: "Andor", Source: "Sonarr", AirsAt: time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)}}},
			blockingCalendarClient{name: "Radarr", started: started, release: release, items: []dashboard.CalendarItem{{Title: "Heat 2", Source: "Radarr", AirsAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)}}},
		)
		done <- result{items: items, err: err}
	}()

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "Sonarr", "Radarr")
	close(release)
	got := <-done
	if got.err != nil || len(got.items) != 2 {
		t.Fatalf("expected both calendar results, got items=%+v err=%v", got.items, got.err)
	}
}

func TestArrCombinersReportCompleteFailure(t *testing.T) {
	queue, queueErr := combineDownloadQueue(context.Background(), func(string, ...any) {}, 10, failingArrClient{name: "Sonarr"}, failingArrClient{name: "Radarr"})
	if queueErr == nil || queue != nil {
		t.Fatalf("queue = %+v, err = %v; want complete failure", queue, queueErr)
	}
	calendar, calendarErr := combineUpcomingCalendar(context.Background(), func(string, ...any) {}, time.Now(), time.Now().Add(time.Hour), 10, failingArrClient{name: "Sonarr"}, failingArrClient{name: "Radarr"})
	if calendarErr == nil || calendar != nil {
		t.Fatalf("calendar = %+v, err = %v; want complete failure", calendar, calendarErr)
	}
}

func TestArrCombinersKeepPartialResults(t *testing.T) {
	success := successfulArrClient{name: "Sonarr"}
	queue, queueErr := combineDownloadQueue(context.Background(), func(string, ...any) {}, 10, success, failingArrClient{name: "Radarr"})
	if queueErr != nil || len(queue) != 1 {
		t.Fatalf("queue = %+v, err = %v; want partial result", queue, queueErr)
	}
	calendar, calendarErr := combineUpcomingCalendar(context.Background(), func(string, ...any) {}, time.Now(), time.Now().Add(time.Hour), 10, success, failingArrClient{name: "Radarr"})
	if calendarErr != nil || len(calendar) != 1 {
		t.Fatalf("calendar = %+v, err = %v; want partial result", calendar, calendarErr)
	}
}

func TestRunConcurrentlyPropagatesWorkerPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected worker panic to be propagated")
		}
	}()
	runConcurrently(func() { panic("boom") })
}

func TestCollectProviderItemsContainsProviderPanic(t *testing.T) {
	items, err := collectProviderItems([]providerLoad[int]{{name: "broken", load: func() ([]int, error) {
		panic("boom")
	}}}, nil)
	if err == nil || items != nil {
		t.Fatalf("items=%v err=%v; want contained provider failure", items, err)
	}
}

type successfulArrClient struct{ name string }

func (c successfulArrClient) Name() string { return c.name }
func (c successfulArrClient) Queue(context.Context, int) ([]dashboard.QueueItem, error) {
	return []dashboard.QueueItem{{Title: "Queue item"}}, nil
}
func (c successfulArrClient) UpcomingWindow(context.Context, time.Time, time.Time, int) ([]dashboard.CalendarItem, error) {
	return []dashboard.CalendarItem{{Title: "Calendar item"}}, nil
}

type failingArrClient struct{ name string }

func (c failingArrClient) Name() string { return c.name }
func (c failingArrClient) Queue(context.Context, int) ([]dashboard.QueueItem, error) {
	return nil, errors.New("unavailable")
}
func (c failingArrClient) UpcomingWindow(context.Context, time.Time, time.Time, int) ([]dashboard.CalendarItem, error) {
	return nil, errors.New("unavailable")
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
