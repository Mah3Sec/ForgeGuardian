// Package queue provides a Redis-backed job queue for build and scan jobs.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/redis/go-redis/v9"
)

const (
	KeyBuildJobs = "fg:jobs:build"
	KeyScanJobs  = "fg:jobs:scan"
)

// BuildJob is the payload pushed to the build queue.
type BuildJob struct {
	PackageVersionID int64              `json:"package_version_id"`
	Package          core.PackageVersion `json:"package"`
	EnqueuedAt       time.Time          `json:"enqueued_at"`
}

// Queue wraps a Redis client.
type Queue struct {
	rdb *redis.Client
}

// New creates a Queue from a Redis URL.
func New(redisURL string) (*Queue, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("queue: parse redis url: %w", err)
	}
	return &Queue{rdb: redis.NewClient(opts)}, nil
}

// Close releases the Redis connection.
func (q *Queue) Close() error { return q.rdb.Close() }

// EnqueueBuild pushes a build job onto the Redis list.
func (q *Queue) EnqueueBuild(ctx context.Context, job BuildJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("queue: marshal build job: %w", err)
	}
	if err := q.rdb.RPush(ctx, KeyBuildJobs, b).Err(); err != nil {
		return fmt.Errorf("queue: rpush build job: %w", err)
	}
	return nil
}

// Ping checks the Redis connection.
func (q *Queue) Ping(ctx context.Context) error {
	return q.rdb.Ping(ctx).Err()
}
