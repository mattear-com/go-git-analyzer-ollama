package handler

import (
	"strconv"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/gofiber/fiber/v3"
)

// AuditHandler handles audit log endpoints.
type AuditHandler struct {
	store *store.PostgresStore
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(store *store.PostgresStore) *AuditHandler {
	return &AuditHandler{store: store}
}

// Register sets up audit routes.
func (h *AuditHandler) Register(router fiber.Router) {
	audit := router.Group("/audit")
	audit.Get("/logs", h.ListLogs)
}

// ListLogs returns audit logs with optional filtering.
func (h *AuditHandler) ListLogs(c fiber.Ctx) error {
	limitStr := c.Query("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	action := c.Query("action", "")

	logs, err := h.store.ListAuditLogs(c.Context(), limit, action)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"count": len(logs),
	})
}
