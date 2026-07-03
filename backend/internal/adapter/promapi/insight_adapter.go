package promapi

import (
	"context"
	"time"

	"beacon/internal/domain/insight"
)

// InsightQuerier adapts the generic Prometheus Client to the insight.Querier
// port, mapping promapi result types to the domain's types so the insight
// package need not depend on this adapter.
type InsightQuerier struct {
	client *Client
}

// NewInsightQuerier wraps a Client as an insight.Querier.
func NewInsightQuerier(client *Client) *InsightQuerier {
	return &InsightQuerier{client: client}
}

var _ insight.Querier = (*InsightQuerier)(nil)

// Query runs an instant query and maps the samples.
func (q *InsightQuerier) Query(ctx context.Context, expr string) ([]insight.Sample, error) {
	samples, err := q.client.QueryVector(ctx, expr)
	if err != nil {
		return nil, err
	}
	out := make([]insight.Sample, 0, len(samples))
	for _, s := range samples {
		out = append(out, insight.Sample{Labels: s.Labels, Value: s.Value})
	}
	return out, nil
}

// QueryRange runs a range query and maps the series.
func (q *InsightQuerier) QueryRange(ctx context.Context, expr string, start, end time.Time, step time.Duration) ([]insight.RangeSeries, error) {
	series, err := q.client.QueryRange(ctx, expr, start, end, step)
	if err != nil {
		return nil, err
	}
	out := make([]insight.RangeSeries, 0, len(series))
	for _, s := range series {
		points := make([]insight.Point, 0, len(s.Points))
		for _, p := range s.Points {
			points = append(points, insight.Point{T: p.T, V: p.V})
		}
		out = append(out, insight.RangeSeries{Labels: s.Labels, Points: points})
	}
	return out, nil
}
