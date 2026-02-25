package middleware

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// AuditWriter defines how audit records are persisted.
type AuditWriter interface {
	WriteAudit(userID, action, resource, resourceID, details, ip, userAgent string) error
}

// AuditMiddleware logs every request for compliance purposes.
func AuditMiddleware(writer AuditWriter) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Capture request data BEFORE handler execution (Fiber reuses context objects)
		method := c.Method()
		path := c.Path()
		ip := c.IP()
		userAgent := c.Get("User-Agent")

		// Execute the handler
		err := c.Next()

		// Extract user info if available
		userID := "anonymous"
		if uc := GetUserContext(c); uc != nil {
			userID = uc.UserID
		}

		// Build audit details with pre-captured values
		statusCode := c.Response().StatusCode()
		details := map[string]interface{}{
			"method":      method,
			"path":        path,
			"status":      statusCode,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		detailsJSON, _ := json.Marshal(details)

		// Write audit log asynchronously — all values are captured, safe to use in goroutine
		go func() {
			if writeErr := writer.WriteAudit(
				userID,
				"http_request",
				"api",
				path,
				string(detailsJSON),
				ip,
				userAgent,
			); writeErr != nil {
				slog.Error("failed to write audit log", "error", writeErr)
			}
		}()

		return err
	}
}
