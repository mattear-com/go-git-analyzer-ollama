package handler

import (
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/service"
	"github.com/gofiber/fiber/v3"
)

// RAGHandler handles RAG chat endpoints.
type RAGHandler struct {
	ragService *service.RAGService
}

// NewRAGHandler creates a new RAG handler.
func NewRAGHandler(ragService *service.RAGService) *RAGHandler {
	return &RAGHandler{ragService: ragService}
}

// Register sets up RAG routes.
func (h *RAGHandler) Register(router fiber.Router) {
	rag := router.Group("/rag")
	rag.Post("/query", h.Query)
}

// Query performs a RAG query over a repository's code.
func (h *RAGHandler) Query(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var body struct {
		RepoID   string `json:"repo_id"`
		Question string `json:"question"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	answer, chunks, err := h.ragService.Query(c.Context(), body.RepoID, body.Question)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Build sources from chunks
	sources := make([]fiber.Map, len(chunks))
	for i, chunk := range chunks {
		sources[i] = fiber.Map{
			"file_path":   chunk.FilePath,
			"content":     chunk.Content,
			"similarity":  chunk.Similarity,
			"chunk_index": chunk.ChunkIndex,
		}
	}

	return c.JSON(fiber.Map{
		"answer":  answer,
		"sources": sources,
	})
}
