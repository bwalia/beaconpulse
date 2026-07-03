package queue

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// HandlerFunc processes a job. Returning an error causes the job to be retried
// (up to MaxRetries) and then dead-lettered.
type HandlerFunc func(ctx context.Context, job Job) error

// ConsumerConfig tunes the consumer.
type ConsumerConfig struct {
	Stream      string
	Group       string
	Consumer    string // unique per worker instance
	MaxRetries  int
	BlockTime   time.Duration // how long XREADGROUP blocks
	ReclaimIdle time.Duration // messages idle longer than this are reclaimed
	BatchSize   int64
}

func (c *ConsumerConfig) withDefaults() {
	if c.Stream == "" {
		c.Stream = DefaultStream
	}
	if c.Group == "" {
		c.Group = DefaultGroup
	}
	if c.Consumer == "" {
		c.Consumer = "worker-1"
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 5
	}
	if c.BlockTime <= 0 {
		c.BlockTime = 5 * time.Second
	}
	if c.ReclaimIdle <= 0 {
		c.ReclaimIdle = 30 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 16
	}
}

// Consumer reads and dispatches jobs from the stream.
type Consumer struct {
	rdb      *redis.Client
	cfg      ConsumerConfig
	log      *slog.Logger
	handlers map[string]HandlerFunc
}

// NewConsumer builds a Consumer.
func NewConsumer(rdb *redis.Client, cfg ConsumerConfig, log *slog.Logger) *Consumer {
	cfg.withDefaults()
	return &Consumer{rdb: rdb, cfg: cfg, log: log, handlers: map[string]HandlerFunc{}}
}

// Register binds a handler to a job type. Must be called before Run.
func (c *Consumer) Register(jobType string, h HandlerFunc) {
	c.handlers[jobType] = h
}

// Run consumes jobs until ctx is cancelled. It first ensures the consumer group
// exists, then interleaves reading new messages with reclaiming stuck ones.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}
	reclaim := time.NewTicker(c.cfg.ReclaimIdle)
	defer reclaim.Stop()

	c.log.Info("job consumer started",
		slog.String("stream", c.cfg.Stream),
		slog.String("group", c.cfg.Group),
		slog.String("consumer", c.cfg.Consumer))

	for {
		select {
		case <-ctx.Done():
			c.log.Info("job consumer stopping")
			return nil
		case <-reclaim.C:
			c.reclaim(ctx)
		default:
			c.readOnce(ctx)
		}
	}
}

func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.cfg.Stream, c.cfg.Group, "$").Err()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

// readOnce blocks for one batch of new messages and processes them.
func (c *Consumer) readOnce(ctx context.Context) {
	res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.cfg.Group,
		Consumer: c.cfg.Consumer,
		Streams:  []string{c.cfg.Stream, ">"},
		Count:    c.cfg.BatchSize,
		Block:    c.cfg.BlockTime,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			return // timeout with no messages, or shutting down
		}
		c.log.Error("xreadgroup failed", slog.String("error", err.Error()))
		time.Sleep(time.Second) // avoid a hot loop if Redis is flapping
		return
	}
	for _, stream := range res {
		for _, msg := range stream.Messages {
			c.process(ctx, msg)
		}
	}
}

// reclaim takes over messages that another (possibly crashed) consumer left
// pending for too long.
func (c *Consumer) reclaim(ctx context.Context) {
	msgs, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   c.cfg.Stream,
		Group:    c.cfg.Group,
		Consumer: c.cfg.Consumer,
		MinIdle:  c.cfg.ReclaimIdle,
		Start:    "0",
		Count:    c.cfg.BatchSize,
	}).Result()
	if err != nil {
		c.log.Error("xautoclaim failed", slog.String("error", err.Error()))
		return
	}
	for _, msg := range msgs {
		c.process(ctx, msg)
	}
}

// process dispatches a single message, acking on success and retrying or
// dead-lettering on failure.
func (c *Consumer) process(ctx context.Context, msg redis.XMessage) {
	job := Job{
		Type:    stringField(msg.Values, fieldType),
		Payload: []byte(stringField(msg.Values, fieldPayload)),
	}
	handler, ok := c.handlers[job.Type]
	if !ok {
		c.log.Warn("no handler for job type; discarding", slog.String("type", job.Type))
		c.ack(ctx, msg.ID)
		return
	}

	if err := handler(ctx, job); err != nil {
		deliveries := c.deliveryCount(ctx, msg.ID)
		if deliveries >= int64(c.cfg.MaxRetries) {
			c.log.Error("job failed permanently; dead-lettering",
				slog.String("type", job.Type),
				slog.Int64("deliveries", deliveries),
				slog.String("error", err.Error()))
			c.deadLetter(ctx, msg)
			c.ack(ctx, msg.ID)
			return
		}
		c.log.Warn("job failed; will retry",
			slog.String("type", job.Type),
			slog.Int64("deliveries", deliveries),
			slog.String("error", err.Error()))
		// Leave the message unacked; the reclaim loop redelivers it after the
		// idle window.
		return
	}
	c.ack(ctx, msg.ID)
}

func (c *Consumer) ack(ctx context.Context, id string) {
	if err := c.rdb.XAck(ctx, c.cfg.Stream, c.cfg.Group, id).Err(); err != nil {
		c.log.Error("xack failed", slog.String("error", err.Error()))
	}
	// Best-effort trim of the acked entry to keep the stream small.
	_ = c.rdb.XDel(ctx, c.cfg.Stream, id).Err()
}

func (c *Consumer) deliveryCount(ctx context.Context, id string) int64 {
	pending, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.cfg.Stream,
		Group:  c.cfg.Group,
		Start:  id,
		End:    id,
		Count:  1,
	}).Result()
	if err != nil || len(pending) == 0 {
		return 1
	}
	return pending[0].RetryCount
}

func (c *Consumer) deadLetter(ctx context.Context, msg redis.XMessage) {
	values := map[string]any{}
	for k, v := range msg.Values {
		values[k] = v
	}
	values["original_id"] = msg.ID
	if err := c.rdb.XAdd(ctx, &redis.XAddArgs{Stream: deadStream, Values: values}).Err(); err != nil {
		c.log.Error("failed to write dead-letter", slog.String("error", err.Error()))
	}
}

func isBusyGroup(err error) bool {
	return err != nil && (err.Error() == "BUSYGROUP Consumer Group name already exists")
}

func stringField(values map[string]any, key string) string {
	if v, ok := values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
