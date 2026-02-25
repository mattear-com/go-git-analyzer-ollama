package handler

import (
	"strings"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/gofiber/fiber/v3"
)

// ReportsHandler handles analysis reports endpoints.
type ReportsHandler struct {
	store       *store.PostgresStore
	vectorStore *store.VectorStore
}

// NewReportsHandler creates a new reports handler.
func NewReportsHandler(s *store.PostgresStore, vs *store.VectorStore) *ReportsHandler {
	return &ReportsHandler{store: s, vectorStore: vs}
}

// Register sets up report routes.
func (h *ReportsHandler) Register(router fiber.Router) {
	reports := router.Group("/reports")
	reports.Get("/", h.ListAll)
	reports.Get("/search", h.Search)
	reports.Get("/:repoId", h.ListByRepo)
	reports.Delete("/:repoId", h.DeleteByRepo)
}

// ListAll returns all analysis results for the current user's repos.
func (h *ReportsHandler) ListAll(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	results, err := h.store.ListAllAnalysisResults(c.Context(), uc.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Also get repos for name mapping
	repos, _ := h.store.ListReposByUser(c.Context(), uc.UserID)
	repoMap := make(map[string]string)
	for _, r := range repos {
		repoMap[r.ID] = r.Name
	}

	return c.JSON(fiber.Map{
		"results":  results,
		"count":    len(results),
		"repo_map": repoMap,
	})
}

// ListByRepo returns all analysis results for a specific repo.
func (h *ReportsHandler) ListByRepo(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	repoID := c.Params("repoId")
	results, err := h.store.ListAnalysisResults(c.Context(), repoID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"results": results, "count": len(results)})
}

// DeleteByRepo deletes all analysis results and embeddings for a repo.
func (h *ReportsHandler) DeleteByRepo(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	repoID := c.Params("repoId")

	// Verify user owns group repo
	repo, err := h.store.GetRepoByID(repoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "repo not found"})
	}
	if repo.UserID != uc.UserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	// Delete embeddings first, then analysis results
	if h.vectorStore != nil {
		if err := h.vectorStore.DeleteEmbeddingsByRepo(c.Context(), repoID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete embeddings: " + err.Error()})
		}
	}

	if err := h.store.DeleteAnalysisResultsByRepo(c.Context(), repoID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete reports: " + err.Error()})
	}

	return c.JSON(fiber.Map{"ok": true, "message": "reports and embeddings deleted"})
}

// Search searches analysis results by strategy or summary text.
func (h *ReportsHandler) Search(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.JSON(fiber.Map{"results": []interface{}{}, "count": 0, "repo_map": map[string]string{}})
	}

	repoID := c.Query("repo_id")

	results, err := h.store.SearchAnalysisResults(c.Context(), uc.UserID, q, repoID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Repo name map
	repos, _ := h.store.ListReposByUser(c.Context(), uc.UserID)
	repoMap := make(map[string]string)
	for _, r := range repos {
		repoMap[r.ID] = r.Name
	}

	return c.JSON(fiber.Map{
		"results":  results,
		"count":    len(results),
		"repo_map": repoMap,
	})
}
