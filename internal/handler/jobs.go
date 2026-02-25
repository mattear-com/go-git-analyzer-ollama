package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// JobStatus represents the current state of an analysis job.
type JobStatus struct {
	ID          string    `json:"id"`
	RepoID      string    `json:"repo_id"`
	Status      string    `json:"status"` // running, complete, error
	Progress    int       `json:"progress"`
	Total       int       `json:"total"`
	Current     string    `json:"current_strategy"`
	Results     []string  `json:"completed_strategies"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// JobTracker manages analysis jobs in memory.
type JobTracker struct {
	mu   sync.RWMutex
	jobs map[string]*JobStatus
	subs map[string][]chan JobStatus // subscribers per job
}

// NewJobTracker creates a new job tracker.
func NewJobTracker() *JobTracker {
	return &JobTracker{
		jobs: make(map[string]*JobStatus),
		subs: make(map[string][]chan JobStatus),
	}
}

// CreateJob creates a new job entry.
func (t *JobTracker) CreateJob(id, repoID string, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.jobs[id] = &JobStatus{
		ID:        id,
		RepoID:    repoID,
		Status:    "running",
		Total:     total,
		Results:   []string{},
		StartedAt: time.Now(),
	}
}

// UpdateJob updates a job and notifies subscribers.
func (t *JobTracker) UpdateJob(id string, strategy string, progress int, status string) {
	t.mu.Lock()
	job, ok := t.jobs[id]
	if !ok {
		t.mu.Unlock()
		return
	}
	job.Progress = progress
	job.Current = strategy
	job.Status = status
	if strategy != "" && status != "error" {
		job.Results = append(job.Results, strategy)
	}
	if status == "complete" || status == "error" {
		job.CompletedAt = time.Now()
	}
	snapshot := *job
	subs := t.subs[id]
	t.mu.Unlock()

	// Notify subscribers
	for _, ch := range subs {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

// GetJob returns a job status.
func (t *JobTracker) GetJob(id string) (*JobStatus, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	job, ok := t.jobs[id]
	if !ok {
		return nil, false
	}
	snapshot := *job
	return &snapshot, true
}

// Subscribe returns a channel that receives job updates.
func (t *JobTracker) Subscribe(id string) chan JobStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch := make(chan JobStatus, 10)
	t.subs[id] = append(t.subs[id], ch)
	return ch
}

// Unsubscribe removes a channel from subscribers.
func (t *JobTracker) Unsubscribe(id string, ch chan JobStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	subs := t.subs[id]
	for i, s := range subs {
		if s == ch {
			t.subs[id] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	close(ch)
}

// JobsHandler handles job-related endpoints.
type JobsHandler struct {
	tracker *JobTracker
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(tracker *JobTracker) *JobsHandler {
	return &JobsHandler{tracker: tracker}
}

// Register sets up job routes.
func (h *JobsHandler) Register(router fiber.Router) {
	jobs := router.Group("/jobs")
	jobs.Get("/:id", h.GetStatus)
	jobs.Get("/:id/stream", h.StreamSSE)
}

// GetStatus returns the current job status.
func (h *JobsHandler) GetStatus(c fiber.Ctx) error {
	id := c.Params("id")
	job, ok := h.tracker.GetJob(id)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "job not found"})
	}
	return c.JSON(job)
}

// StreamSSE streams job updates via Server-Sent Events.
func (h *JobsHandler) StreamSSE(c fiber.Ctx) error {
	id := c.Params("id")

	job, ok := h.tracker.GetJob(id)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "job not found"})
	}

	// If already complete, just return the final status
	if job.Status == "complete" || job.Status == "error" {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		data, _ := json.Marshal(job)
		return c.SendString(fmt.Sprintf("event: %s\ndata: %s\n\n", job.Status, string(data)))
	}

	ch := h.tracker.Subscribe(id)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer h.tracker.Unsubscribe(id, ch)

		// Send initial status
		data, _ := json.Marshal(job)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", string(data))
		w.Flush()

		timeout := time.After(5 * time.Minute)
		for {
			select {
			case update, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(update)
				eventType := "progress"
				if update.Status == "complete" || update.Status == "error" {
					eventType = update.Status
				}
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(data))
				w.Flush()

				if update.Status == "complete" || update.Status == "error" {
					return
				}
			case <-timeout:
				slog.Warn("SSE timeout", "job_id", id)
				return
			}
		}
	})
}
