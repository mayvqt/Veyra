package seerr

type Quota struct {
	Limit           int
	Used            int
	Remaining       int
	Unlimited       bool
	MovieLimit      int
	MovieUsed       int
	MovieRemaining  int
	MovieUnlimited  bool
	SeriesLimit     int
	SeriesUsed      int
	SeriesRemaining int
	SeriesUnlimited bool
}

type UserIdentity struct {
	ID          int
	Username    string
	DisplayName string
}

type SearchResult struct {
	ID         int            `json:"id"`
	MediaType  string         `json:"mediaType"`
	Title      string         `json:"title"`
	Year       string         `json:"year,omitempty"`
	Overview   string         `json:"overview,omitempty"`
	PosterPath string         `json:"posterPath,omitempty"`
	PosterURL  string         `json:"posterUrl,omitempty"`
	Status     string         `json:"status"`
	CanRequest bool           `json:"canRequest"`
	OpenURL    string         `json:"openUrl,omitempty"`
	Seasons    []SeasonOption `json:"seasons,omitempty"`
}

type SeasonOption struct {
	Number       int    `json:"number"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episodeCount,omitempty"`
}

type CreateRequestInput struct {
	MediaID   int
	MediaType string
	UserID    int
	Is4K      bool
	Seasons   []int
}

type CreatedRequest struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type requestResp struct {
	Results []map[string]any `json:"results"`
}

type userDTO struct {
	ID               int    `json:"id"`
	DisplayName      string `json:"displayName"`
	Username         string `json:"username"`
	PlexUsername     string `json:"plexUsername"`
	JellyfinUsername string `json:"jellyfinUsername"`
	Email            string `json:"email"`
}

type createdRequestDTO struct {
	ID     int `json:"id"`
	Status int `json:"status"`
}
