package seerr

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/testutil"
)

func TestTVSeasonEligibilityUsesStandardRequestsAndAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, details string
		status        int
		want          []int
	}{
		{"partial", `{"seasons":[{"seasonNumber":1},{"seasonNumber":2},{"seasonNumber":3}],"mediaInfo":{"requests":[{"status":1,"is4k":false,"seasons":[{"seasonNumber":1}]},{"status":2,"is4k":true,"seasons":[{"seasonNumber":2}]}],"seasons":[{"seasonNumber":3,"status":5}]}}`, 200, []int{2}},
		{"exhausted", `{"seasons":[{"seasonNumber":1}],"mediaInfo":{"seasons":[{"seasonNumber":1,"status":5}]}}`, 200, nil},
		{"outage", `{}`, 503, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient("http://seerr.invalid", "", "key")
			c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				body, status := tc.details, tc.status
				if r.URL.Path == "/api/v1/search" {
					body, status = `{"results":[{"id":42,"mediaType":"tv","name":"Show","mediaInfo":{"status":3,"requests":[{"status":1}]}}]}`, 200
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			got, err := c.Search(context.Background(), "Show", 6)
			if err != nil || len(got) != 1 {
				t.Fatalf("search: %v %v", got, err)
			}
			if got[0].CanRequest != (len(tc.want) > 0) || len(got[0].Seasons) != len(tc.want) {
				t.Fatalf("eligibility: %+v", got[0])
			}
			for i, season := range tc.want {
				if got[0].Seasons[i].Number != season {
					t.Fatalf("wrong choices: %+v", got[0])
				}
			}
			if tc.status == 503 && got[0].Status != "Season details unavailable" {
				t.Fatal("outage presented as no eligible seasons")
			}
		})
	}
}

func TestSeasonFilteringMatchesSeerrStatusRules(t *testing.T) {
	for status := 1; status <= 6; status++ {
		payload := map[string]any{"mediaInfo": map[string]any{"seasons": []any{map[string]any{"seasonNumber": 1, "status": status}}}}
		if got := unavailableStandardSeasons(payload)[1]; got != (status != 1 && status != 6) {
			t.Fatalf("media status %d blocked=%v", status, got)
		}
		payload = map[string]any{"mediaInfo": map[string]any{"requests": []any{map[string]any{"status": status, "is4k": false, "seasons": []any{map[string]any{"seasonNumber": 1}}}}}}
		if got := unavailableStandardSeasons(payload)[1]; got != (status != 3 && status != 5) {
			t.Fatalf("request status %d blocked=%v", status, got)
		}
	}
}

func TestCreateRequestRejectsNoOpAndMissingIdentity(t *testing.T) {
	c := NewClient("http://seerr.invalid", "", "key")
	calls := 0
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 202, Body: io.NopCloser(strings.NewReader(`{"message":"No seasons available to request"}`)), Header: make(http.Header)}, nil
	})}
	in := CreateRequestInput{MediaID: 42, MediaType: "tv", Seasons: []int{2}}
	if _, err := c.CreateRequest(context.Background(), in); err != ErrUserNotLinked || calls != 0 {
		t.Fatalf("unlinked: calls=%d err=%v", calls, err)
	}
	in.UserID = 7
	if got, err := c.CreateRequest(context.Background(), in); err != ErrRequestConflict || got.ID != 0 {
		t.Fatalf("no-op returned success: %+v %v", got, err)
	}
}
