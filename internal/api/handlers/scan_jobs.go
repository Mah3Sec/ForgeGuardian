package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ScanJobStatus is the lifecycle state of an async scan job.
type ScanJobStatus string

const (
	ScanJobQueued   ScanJobStatus = "queued"
	ScanJobRunning  ScanJobStatus = "running"
	ScanJobComplete ScanJobStatus = "complete"
	ScanJobFailed   ScanJobStatus = "failed"
)

// ScanJob tracks the state of one async scan request.
type ScanJob struct {
	ID        string        `json:"job_id"`
	Status    ScanJobStatus `json:"status"`
	Result    any           `json:"result,omitempty"`
	Error     string        `json:"error,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// scanJobWorkers bounds how many scans run concurrently — an unbounded
// goroutine-per-request pool would let a burst of dashboard clicks starve
// the box running grype/trivy/semgrep for every scan at once.
const scanJobWorkers = 4

// jobRegistry tracks in-flight and completed scan jobs in memory and runs
// them on a small fixed worker pool. Jobs are not persisted here — final
// scan results are still written to Postgres via persistScanResult when a
// DB is configured, same as before this existed.
type jobRegistry struct {
	mu   sync.RWMutex
	jobs map[string]*ScanJob
	work chan func()
}

func newJobRegistry() *jobRegistry {
	r := &jobRegistry{
		jobs: make(map[string]*ScanJob),
		work: make(chan func(), 256),
	}
	for i := 0; i < scanJobWorkers; i++ {
		go r.runWorker()
	}
	return r
}

func (r *jobRegistry) runWorker() {
	for fn := range r.work {
		fn()
	}
}

func newJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// submit creates a queued job and schedules fn to run on the worker pool.
// fn's return value becomes the job's Result on success, or its Error on
// failure. Returns immediately — fn runs asynchronously.
func (r *jobRegistry) submit(fn func() (any, error)) *ScanJob {
	job := &ScanJob{
		ID:        newJobID(),
		Status:    ScanJobQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	r.mu.Lock()
	r.jobs[job.ID] = job
	r.mu.Unlock()

	r.work <- func() {
		r.mu.Lock()
		job.Status = ScanJobRunning
		job.UpdatedAt = time.Now()
		r.mu.Unlock()

		result, err := fn()

		r.mu.Lock()
		if err != nil {
			job.Status = ScanJobFailed
			job.Error = err.Error()
		} else {
			job.Status = ScanJobComplete
			job.Result = result
		}
		job.UpdatedAt = time.Now()
		r.mu.Unlock()
	}

	return job
}

func (r *jobRegistry) get(id string) (*ScanJob, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.jobs[id]
	if !ok {
		return nil, false
	}
	// Return a copy so callers reading concurrently with the worker don't race.
	cp := *job
	return &cp, true
}

// GetJobStatus returns the current state of an async scan job.
// GET /api/v1/jobs/:id
func (h *Handler) GetJobStatus(c *gin.Context) {
	job, ok := h.jobs.get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown job id"})
		return
	}
	c.JSON(http.StatusOK, job)
}
