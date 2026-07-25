package dashboard

import "time"

type MediaItem struct {
	Title    string
	Type     string
	Year     int
	AddedAt  time.Time
	ImageURL string
	OpenURL  string
}

type RequestItem struct {
	Title     string
	Status    string
	StatusKey string
	Media     string
	Created   time.Time
	User      string
	Lifecycle []RequestLifecycleStep
}

type RequestLifecycleStep struct {
	Label string
	Key   string
	State string
}

type QueueItem struct {
	Title     string
	Subtitle  string
	Source    string
	Kind      string
	Status    string
	StatusKey string
	Progress  int
	TimeLeft  string
	Client    string
	Protocol  string
	SizeLeft  string
	SortTime  time.Time
	SortIndex int
}

type CalendarItem struct {
	Title        string
	Subtitle     string
	Kind         string
	Source       string
	AirsAt       time.Time
	Availability string
}
