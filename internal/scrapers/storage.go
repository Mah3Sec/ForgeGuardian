package scrapers

import (
	"context"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/scrapers/queue"
	"github.com/mah3sec/forgeguardian/internal/scrapers/store"
)

// VersionStore is the interface used by the orchestrator to persist and queue versions.
type VersionStore interface {
	GetLastRun(ctx context.Context, ecosystem string) (time.Time, error)
	SetLastRun(ctx context.Context, ecosystem string, t time.Time, count int) error
	UpsertVersion(ctx context.Context, pv core.PackageVersion) (id int64, isNew bool, err error)
	EnqueueBuild(ctx context.Context, id int64, pv core.PackageVersion) error
	Close()
}

// versionStore is the production implementation backed by PostgreSQL + Redis.
type versionStore struct {
	db *store.Store
	q  *queue.Queue
}

// NewVersionStore creates a VersionStore backed by PostgreSQL (connStr) and Redis (redisURL).
func NewVersionStore(ctx context.Context, connStr, redisURL string) (VersionStore, error) {
	db, err := store.New(ctx, connStr)
	if err != nil {
		return nil, err
	}
	q, err := queue.New(redisURL)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &versionStore{db: db, q: q}, nil
}

func (s *versionStore) GetLastRun(ctx context.Context, ecosystem string) (time.Time, error) {
	return s.db.GetScraperLastRun(ctx, ecosystem)
}

func (s *versionStore) SetLastRun(ctx context.Context, ecosystem string, t time.Time, count int) error {
	return s.db.SetScraperLastRun(ctx, ecosystem, t, count)
}

func (s *versionStore) UpsertVersion(ctx context.Context, pv core.PackageVersion) (int64, bool, error) {
	id, isNew, err := s.db.UpsertPackageVersion(ctx, pv)
	if err != nil {
		return 0, false, err
	}
	if isNew {
		// Keep the packages table in sync
		_ = s.db.UpsertPackage(ctx, pv.Ecosystem, pv.Name, pv.Version)
	}
	return id, isNew, nil
}

func (s *versionStore) EnqueueBuild(ctx context.Context, id int64, pv core.PackageVersion) error {
	return s.q.EnqueueBuild(ctx, queue.BuildJob{
		PackageVersionID: id,
		Package:          pv,
	})
}

func (s *versionStore) Close() {
	s.db.Close()
	_ = s.q.Close()
}
