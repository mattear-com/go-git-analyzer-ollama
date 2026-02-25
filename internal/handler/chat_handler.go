package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
	"github.com/gofiber/fiber/v3"
)

// ChatHandler handles per-repo chat with Ollama.
type ChatHandler struct {
	ai    port.AIProvider
	store *store.PostgresStore
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(ai port.AIProvider, pgStore *store.PostgresStore) *ChatHandler {
	return &ChatHandler{ai: ai, store: pgStore}
}

// Register sets up chat routes.
func (h *ChatHandler) Register(router fiber.Router) {
	chat := router.Group("/chat")
	chat.Post("/:repoId", h.Chat)
}

// Chat handles a chat message about a specific repo.
func (h *ChatHandler) Chat(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	repoID := c.Params("repoId")

	var body struct {
		Message string `json:"message"`
		History []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"history"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	// Get repo info
	repo, err := h.store.GetRepoByID(repoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "repo not found"})
	}

	// Get latest analysis results for context
	results, _ := h.store.ListAnalysisResults(c.Context(), repoID)

	// Build context from analysis results
	var analysisContext []string
	analysisContext = append(analysisContext, fmt.Sprintf("Repository: %s\nURL: %s", repo.Name, repo.URL))

	for _, r := range results {
		if len(analysisContext) > 5 {
			break // Limit context
		}
		analysisContext = append(analysisContext, fmt.Sprintf("=== Analysis: %s (score: %.1f) ===\n%s", r.Strategy, r.Score, truncate(r.Summary, 2000)))
	}

	systemPrompt := fmt.Sprintf(`You are CodeLens AI, an expert assistant for the repository "%s". 
You have access to analysis results from architecture, code quality, functionality, and DevOps reviews.
Answer questions about the codebase based on the analysis data provided.
Be specific, reference actual files and patterns found.
Use Markdown formatting in your responses. Include Mermaid diagrams when appropriate.
Be concise but thorough.`, repo.Name)

	// Build conversation with history
	userMessage := body.Message
	if len(body.History) > 0 {
		// Append recent history as context
		for _, h := range body.History {
			if len(h.Content) > 500 {
				continue
			}
			analysisContext = append(analysisContext, fmt.Sprintf("[%s]: %s", h.Role, h.Content))
		}
	}

	chatCtx, cancel := context.WithTimeout(c.Context(), 2*time.Minute)
	defer cancel()

	response, err := h.ai.Chat(chatCtx, systemPrompt, userMessage, analysisContext)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "AI failed: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"response": response,
		"repo_id":  repoID,
	})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
