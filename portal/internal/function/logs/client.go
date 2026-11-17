// Package logs streams function runtime and ingestor logs from Loki.
package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLimit        = 100
	maxLimit            = 1000
	defaultPollInterval = 2 * time.Second
	defaultLookback     = 15 * time.Minute
	requestTimeout      = 10 * time.Second
)

// Config controls Loki discovery and tailing. Query, when set, takes
// precedence over Namespace and Apps and must be a complete log selector.
type Config struct {
	URL          string
	Namespace    string
	Apps         []string
	Query        string
	Limit        int
	PollInterval time.Duration
	Lookback     time.Duration
}

// Client tails matching Loki log streams over the HTTP query_range API.
type Client struct {
	baseURL      string
	query        string
	limit        int
	pollInterval time.Duration
	lookback     time.Duration
	httpClient   *http.Client
}

// Entry is one Loki log line and its stream labels.
type Entry struct {
	Timestamp time.Time         `json:"timestamp"`
	Line      string            `json:"line"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func NewClient(cfg Config) (*Client, error) {
	baseURL, err := url.Parse(strings.TrimSpace(cfg.URL))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, fmt.Errorf("loki URL must be an absolute HTTP(S) URL")
	}

	if cfg.Limit == 0 {
		cfg.Limit = defaultLimit
	}
	if cfg.Limit < 0 || cfg.Limit > maxLimit {
		return nil, fmt.Errorf("loki limit must be between 1 and %d", maxLimit)
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.PollInterval < 0 {
		return nil, fmt.Errorf("loki poll interval must be positive")
	}
	if cfg.Lookback == 0 {
		cfg.Lookback = defaultLookback
	}
	if cfg.Lookback < 0 {
		return nil, fmt.Errorf("loki lookback must be positive")
	}

	query := strings.TrimSpace(cfg.Query)
	if query == "" {
		query, err = selector(cfg.Namespace, cfg.Apps)
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		baseURL:      strings.TrimRight(baseURL.String(), "/"),
		query:        query,
		limit:        cfg.Limit,
		pollInterval: cfg.PollInterval,
		lookback:     cfg.Lookback,
		httpClient:   &http.Client{},
	}, nil
}

func selector(namespace string, apps []string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "", fmt.Errorf("loki namespace is required")
	}
	var patterns []string
	seen := map[string]bool{}
	for _, app := range apps {
		app = strings.TrimSpace(app)
		if app == "" || seen[app] {
			continue
		}
		seen[app] = true
		patterns = append(patterns, regexp.QuoteMeta(app))
	}
	if len(patterns) == 0 {
		return "", fmt.Errorf("at least one loki app label is required")
	}
	labels := fmt.Sprintf("namespace=%s,app=~%s", strconv.Quote(namespace), strconv.Quote("^(?:"+strings.Join(patterns, "|")+")$"))
	return "{" + labels + "}", nil
}

type queryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (c *Client) queryRange(ctx context.Context, start, end time.Time, direction string) ([]Entry, error) {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u, err := url.Parse(c.baseURL + "/loki/api/v1/query_range")
	if err != nil {
		return nil, fmt.Errorf("build loki query URL: %w", err)
	}
	params := u.Query()
	params.Set("query", c.query)
	params.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	params.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	params.Set("limit", strconv.Itoa(c.limit))
	params.Set("direction", direction)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create loki request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("loki returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload queryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode loki response: %w", err)
	}
	if payload.Status != "success" {
		if payload.Error == "" {
			payload.Error = "query status " + payload.Status
		}
		return nil, fmt.Errorf("loki query failed: %s", payload.Error)
	}

	var entries []Entry
	for _, stream := range payload.Data.Result {
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}
			nanoseconds, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				continue
			}
			entries = append(entries, Entry{
				Timestamp: time.Unix(0, nanoseconds).UTC(),
				Line:      value[1],
				Labels:    stream.Stream,
			})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}
