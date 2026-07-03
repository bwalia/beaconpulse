// Package queue implements a reliable background-job queue on top of Redis
// Streams with a consumer group. Jobs survive process crashes: an unacknowledged
// message stays pending and is re-delivered (via XAUTOCLAIM) to another worker,
// with a bounded retry count before it is moved to a dead-letter stream. This
// backs Beacon's asynchronous work such as control-plane reconciliation.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Default stream and group names.
const (
	DefaultStream = "beacon:jobs"
	DefaultGroup  = "beacon-workers"
	deadStream    = "beacon:jobs:dead"

	fieldType    = "type"
	fieldPayload = "payload"
)

// Job is a unit of asynchronous work.
type Job struct {
	Type    string
	Payload json.RawMessage
}

// Queue is the producer side: it enqueues jobs onto the stream.
type Queue struct {
	rdb    *redis.Client
	stream string
}

// NewQueue builds a producer for the given stream.
func NewQueue(rdb *redis.Client, stream string) *Queue {
	if stream == "" {
		stream = DefaultStream
	}
	return &Queue{rdb: rdb, stream: stream}
}

// Enqueue appends a job. payload is JSON-encoded; pass nil for jobs without a
// payload.
func (q *Queue) Enqueue(ctx context.Context, jobType string, payload any) error {
	var raw []byte
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("queue: marshal payload: %w", err)
		}
		raw = b
	}
	if err := q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{fieldType: jobType, fieldPayload: string(raw)},
		// Cap the stream so a long-running system does not grow unbounded; acked
		// entries are trimmed opportunistically.
		Approx: true,
		MaxLen: 100000,
	}).Err(); err != nil {
		return fmt.Errorf("queue: xadd: %w", err)
	}
	return nil
}
