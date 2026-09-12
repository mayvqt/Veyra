package seerr

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mayvqt/veyra/internal/integrations"
)

func (c *Client) UserQuotaForUser(ctx context.Context, user UserIdentity) (*Quota, error) {
	if user.ID <= 0 {
		return nil, ErrUserNotLinked
	}
	return c.userQuotaFrom(ctx, fmt.Sprintf("/api/v1/user/%d/quota", user.ID))
}

func (c *Client) userQuotaFrom(ctx context.Context, path string) (*Quota, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, integrations.NewHTTPStatusError("Seerr", "quota", resp.StatusCode)
	}
	var payload map[string]any
	if err := decodeLimitedSeerrJSON(resp.Body, &payload); err != nil {
		return nil, err
	}
	limit, lok := findInt(payload, "requestLimit")
	used, uok := findInt(payload, "requestCount")

	mLimit, mLok := findInt(payload, "movieRequestLimit")
	mUsed, mUok := findInt(payload, "movieRequestCount")
	sLimit, sLok := findInt(payload, "seriesRequestLimit")
	sUsed, sUok := findInt(payload, "seriesRequestCount")
	if !sLok || !sUok {
		sLimit, sLok = findInt(payload, "tvRequestLimit")
		sUsed, sUok = findInt(payload, "tvRequestCount")
	}
	if !mLok || !mUok {
		mLimit, mLok = nestedQuotaInt(payload, "movie", "limit")
		mUsed, mUok = nestedQuotaInt(payload, "movie", "used")
	}
	if !sLok || !sUok {
		sLimit, sLok = nestedQuotaInt(payload, "tv", "limit")
		sUsed, sUok = nestedQuotaInt(payload, "tv", "used")
	}

	q := &Quota{}
	if lok && uok && limit == 0 {
		q.Unlimited = true
		q.Used = used
	}
	if lok && uok && limit > 0 && used >= 0 {
		q.Limit = limit
		q.Used = used
		q.Remaining = max(0, limit-used)
	}
	if mLok && mUok && mLimit >= 0 && mUsed >= 0 {
		q.MovieLimit = mLimit
		q.MovieUsed = mUsed
		q.MovieUnlimited = mLimit == 0
		if mLimit > 0 {
			q.MovieRemaining = max(0, mLimit-mUsed)
		}
	}
	if sLok && sUok && sLimit >= 0 && sUsed >= 0 {
		q.SeriesLimit = sLimit
		q.SeriesUsed = sUsed
		q.SeriesUnlimited = sLimit == 0
		if sLimit > 0 {
			q.SeriesRemaining = max(0, sLimit-sUsed)
		}
	}

	if q.Limit == 0 && !q.Unlimited && q.MovieLimit == 0 && !q.MovieUnlimited && q.SeriesLimit == 0 && !q.SeriesUnlimited {
		return nil, fmt.Errorf("quota fields not present")
	}
	if q.Limit == 0 && (q.MovieLimit > 0 || q.SeriesLimit > 0) {
		q.Limit = q.MovieLimit + q.SeriesLimit
		q.Used = q.MovieUsed + q.SeriesUsed
		q.Remaining = q.MovieRemaining + q.SeriesRemaining
	}
	return q, nil
}

func nestedQuotaInt(payload map[string]any, group, field string) (int, bool) {
	n, ok := intPath(payload, group, field)
	return n, ok
}

func findInt(v any, target string) (int, bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			if k == target {
				switch n := vv.(type) {
				case float64:
					return int(n), true
				case int:
					return n, true
				}
			}
			if n, ok := findInt(vv, target); ok {
				return n, true
			}
		}
	case []any:
		for _, vv := range x {
			if n, ok := findInt(vv, target); ok {
				return n, true
			}
		}
	}
	return 0, false
}
