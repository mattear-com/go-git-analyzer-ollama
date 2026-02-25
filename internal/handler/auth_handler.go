package handler

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/service"
	"github.com/gofiber/fiber/v3"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService *service.AuthService
	frontendURL string
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(authService *service.AuthService, frontendURL string) *AuthHandler {
	return &AuthHandler{authService: authService, frontendURL: frontendURL}
}

// Register sets up auth routes.
func (h *AuthHandler) Register(app *fiber.App) {
	auth := app.Group("/api/v1/auth")
	auth.Get("/:provider/login", h.Login)
	auth.Get("/:provider/callback", h.Callback)

	// Shared callback route — both Google and GitHub redirect here
	// Provider is encoded in the state param as "provider:random"
	app.Get("/auth/callback", h.CallbackDirect)
}

// Login redirects to the OAuth2 provider's consent screen.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	provider := c.Params("provider")
	// Encode provider name into state so CallbackDirect knows which provider to use
	state := provider + ":" + generateState()

	authURL, err := h.authService.GetAuthURL(provider, state)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Redirect().To(authURL)
}

// Callback handles the OAuth2 callback from the provider (via /api/v1/auth/:provider/callback).
func (h *AuthHandler) Callback(c fiber.Ctx) error {
	provider := c.Params("provider")
	code := c.Query("code")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing authorization code",
		})
	}

	jwt, user, err := h.authService.HandleCallback(c.Context(), provider, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	redirectURL := h.frontendURL + "/auth/callback?token=" + jwt + "&name=" + user.Name
	return c.Redirect().To(redirectURL)
}

// CallbackDirect handles the shared /auth/callback route.
// Extracts the provider from the state param (format: "provider:randomhex").
func (h *AuthHandler) CallbackDirect(c fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing authorization code",
		})
	}

	// Extract provider from state ("github:abc123" → "github")
	provider := "google" // default fallback
	if state != "" {
		if parts := strings.SplitN(state, ":", 2); len(parts) == 2 {
			provider = parts[0]
		}
	}

	jwt, user, err := h.authService.HandleCallback(c.Context(), provider, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	redirectURL := h.frontendURL + "/auth/callback?token=" + jwt + "&name=" + user.Name
	return c.Redirect().To(redirectURL)
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
