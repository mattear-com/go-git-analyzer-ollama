package handler

import (
	"encoding/json"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/gofiber/fiber/v3"
)

// StreamHandler handles Server-Sent Events for real-time log streaming.
type StreamHandler struct {
	store *store.PostgresStore
}

// NewStreamHandler creates a new SSE stream handler.
func NewStreamHandler(store *store.PostgresStore) *StreamHandler {
	return &StreamHandler{store: store}
}

// Register sets up streaming routes.
func (h *StreamHandler) Register(router fiber.Router) {
	router.Get("/stream/logs", h.StreamLogs)
}

// StreamLogs returns the latest audit logs for real-time polling.
// In production, this should be upgraded to WebSocket or SSE with Fiber's streaming API.
func (h *StreamHandler) StreamLogs(c fiber.Ctx) error {
	c.Set("Content-Type", "application/json")
	c.Set("Cache-Control", "no-cache")

	logs, err := h.store.ListAuditLogs(c.Context(), 50, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type logEntry struct {
		Timestamp string `json:"timestamp"`
		Action    string `json:"action"`
		Resource  string `json:"resource"`
		UserID    string `json:"user_id"`
		Details   string `json:"details"`
	}

	entries := make([]logEntry, len(logs))
	for i, l := range logs {
		entries[i] = logEntry{
			Timestamp: l.CreatedAt.Format(time.RFC3339),
			Action:    l.Action,
			Resource:  l.Resource,
			UserID:    l.UserID,
			Details:   l.Details,
		}
	}

	result, _ := json.Marshal(fiber.Map{
		"logs":  entries,
		"count": len(entries),
	})

	return c.Send(result)
}
