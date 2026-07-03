// Package promapi is a minimal read-only client for the Prometheus HTTP query
// API. Beacon uses it to read probe results (probe_success, probe_duration_...)
// back out of Prometheus so it can reflect live monitor status in the dashboard.
// It intentionally implements only the instant-query endpoint it needs.
package promapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Sample is one series returned by an instant vector query: its label set and
// scalar value at the evaluation timestamp.
type Sample struct {
	Labels    map[string]string
	Value     float64
	Timestamp time.Time
}

// Point is a single (timestamp, value) pair in a range query result.
type Point struct {
	T time.Time
	V float64
}

// Series is one labelled time series returned by a range query.
type Series struct {
	Labels map[string]string
	Points []Point
}

// Client queries a Prometheus server.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a Client for the given Prometheus base URL (e.g. http://prometheus:9090).
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// promResponse mirrors the subset of the Prometheus query API response we use.
type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"` // [ <unix_ts float>, "<value string>" ]
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

// QueryVector runs an instant query expected to return a vector and decodes the
// samples. A non-vector or API error yields an error.
func (c *Client) QueryVector(ctx context.Context, query string) ([]Sample, error) {
	endpoint := c.baseURL + "/api/v1/query?query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("promapi: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("promapi: request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("promapi: query returned HTTP %d", resp.StatusCode)
	}

	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("promapi: decode: %w", err)
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("promapi: query failed: %s: %s", pr.ErrorType, pr.Error)
	}
	if pr.Data.ResultType != "vector" {
		return nil, fmt.Errorf("promapi: expected vector result, got %q", pr.Data.ResultType)
	}

	out := make([]Sample, 0, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		s := Sample{Labels: r.Metric}
		if ts, ok := r.Value[0].(float64); ok {
			s.Timestamp = time.Unix(int64(ts), 0).UTC()
		}
		if raw, ok := r.Value[1].(string); ok {
			if v, err := strconv.ParseFloat(raw, 64); err == nil {
				s.Value = v
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// rangeResponse mirrors the matrix result of the query_range endpoint.
type rangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][2]any          `json:"values"` // [ [ts, "val"], … ]
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

// QueryRange runs a range query and decodes the resulting matrix into series.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]Series, error) {
	stepSecs := int64(step.Seconds())
	if stepSecs < 1 {
		stepSecs = 1
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", strconv.FormatInt(stepSecs, 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/query_range?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("promapi: build range request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("promapi: range request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("promapi: range query returned HTTP %d", resp.StatusCode)
	}

	var rr rangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, fmt.Errorf("promapi: decode range: %w", err)
	}
	if rr.Status != "success" {
		return nil, fmt.Errorf("promapi: range query failed: %s: %s", rr.ErrorType, rr.Error)
	}

	out := make([]Series, 0, len(rr.Data.Result))
	for _, r := range rr.Data.Result {
		s := Series{Labels: r.Metric, Points: make([]Point, 0, len(r.Values))}
		for _, v := range r.Values {
			var p Point
			if ts, ok := v[0].(float64); ok {
				p.T = time.Unix(int64(ts), 0).UTC()
			}
			if raw, ok := v[1].(string); ok {
				if f, err := strconv.ParseFloat(raw, 64); err == nil {
					p.V = f
				}
			}
			s.Points = append(s.Points, p)
		}
		out = append(out, s)
	}
	return out, nil
}
