package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/vcs"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/service"
	"github.com/gofiber/fiber/v3"
)

// RepoHandler handles repository CRUD and GitHub integration.
// RepoEvent represents a repo status change sent via SSE.
type RepoEvent struct {
	RepoID string `json:"repo_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// RepoEventBus broadcasts repo status changes to SSE subscribers.
type RepoEventBus struct {
	mu   sync.RWMutex
	subs []chan RepoEvent
}

func NewRepoEventBus() *RepoEventBus {
	return &RepoEventBus{}
}

func (b *RepoEventBus) Publish(evt RepoEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *RepoEventBus) Subscribe() chan RepoEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan RepoEvent, 10)
	b.subs = append(b.subs, ch)
	return ch
}

func (b *RepoEventBus) Unsubscribe(ch chan RepoEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s == ch {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			break
		}
	}
	close(ch)
}

type RepoHandler struct {
	repoService *service.RepoService
	store       *store.PostgresStore
	gitVCS      *vcs.GitProvider
	httpClient  *http.Client
	events      *RepoEventBus
}

// NewRepoHandler creates a new repo handler.
func NewRepoHandler(repoService *service.RepoService, store *store.PostgresStore, gitVCS *vcs.GitProvider) *RepoHandler {
	return &RepoHandler{
		repoService: repoService,
		store:       store,
		gitVCS:      gitVCS,
		httpClient:  &http.Client{},
		events:      NewRepoEventBus(),
	}
}

// Register sets up repo routes on a protected group.
func (h *RepoHandler) Register(api fiber.Router) {
	repos := api.Group("/repos")
	repos.Get("/", h.List)
	repos.Post("/", h.Create)
	repos.Get("/search", h.Search)
	repos.Get("/events", h.StreamEvents)
	repos.Get("/github", h.ListGitHub)
	repos.Post("/clone", h.Clone)
	repos.Get("/:id/gitgraph", h.GitGraph)
}

// List returns repos from our local database for the current user.
func (h *RepoHandler) List(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	repos, err := h.store.ListReposByUser(c.Context(), uc.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"repos": repos, "count": len(repos)})
}

// Search searches repos by name or URL.
func (h *RepoHandler) Search(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.JSON(fiber.Map{"repos": []interface{}{}, "count": 0})
	}

	repos, err := h.store.SearchReposByUser(c.Context(), uc.UserID, q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"repos": repos, "count": len(repos)})
}

// Create adds a new repo record (manual URL entry).
func (h *RepoHandler) Create(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var body struct {
		URL    string `json:"url"`
		Name   string `json:"name"`
		Branch string `json:"branch"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if body.Branch == "" {
		body.Branch = "main"
	}

	repo := &domain.Repo{
		UserID:        uc.UserID,
		Name:          body.Name,
		URL:           body.URL,
		DefaultBranch: body.Branch,
		Status:        "pending",
	}

	created, err := h.store.CreateRepo(c.Context(), repo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(created)
}

// GitHubRepo represents a repo from the GitHub API.
type GitHubRepo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Language      string `json:"language"`
	Stars         int    `json:"stargazers_count"`
	UpdatedAt     string `json:"updated_at"`
}

// ListGitHub lists the user's GitHub repos using the stored access token.
func (h *RepoHandler) ListGitHub(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Get user with access token
	user, err := h.store.GetUserByID(c.Context(), uc.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	if user.AccessToken == "" || user.Provider != "github" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no GitHub access token — please login with GitHub",
		})
	}

	// Fetch repos from GitHub API (paginated, up to 100)
	page := queryInt(c, "page", 1)
	perPage := queryInt(c, "per_page", 100)

	url := fmt.Sprintf("https://api.github.com/user/repos?visibility=all&sort=updated&per_page=%d&page=%d", perPage, page)

	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, url, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "create request"})
	}
	req.Header.Set("Authorization", "Bearer "+user.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "github api error"})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":  "github api error",
			"status": resp.StatusCode,
			"body":   string(body),
		})
	}

	var repos []GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "decode github response"})
	}

	return c.JSON(fiber.Map{"repos": repos, "count": len(repos)})
}

// Clone clones a GitHub repo into our system.
func (h *RepoHandler) Clone(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var body struct {
		URL    string `json:"url"`
		Name   string `json:"name"`
		Branch string `json:"branch"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if body.Branch == "" {
		body.Branch = "main"
	}

	// Get user's access token for authenticated cloning
	user, err := h.store.GetUserByID(c.Context(), uc.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	// Inject token into clone URL for private repos: https://x-access-token:TOKEN@github.com/...
	cloneURL := body.URL
	if user.AccessToken != "" && strings.Contains(cloneURL, "github.com") {
		cloneURL = strings.Replace(cloneURL, "https://github.com", "https://x-access-token:"+user.AccessToken+"@github.com", 1)
	}

	// Create repo record (store the original URL, not the one with token)
	repo := &domain.Repo{
		UserID:        uc.UserID,
		Name:          body.Name,
		URL:           body.URL,
		DefaultBranch: body.Branch,
		Status:        "cloning",
	}

	created, err := h.store.CreateRepo(c.Context(), repo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Clone asynchronously using the authenticated URL
	authURL := cloneURL
	go func() {
		if cloneErr := h.repoService.CloneRepoWithURL(created, authURL); cloneErr != nil {
			_ = h.store.UpdateRepoStatus(context.Background(), created.ID, "error", "")
			h.events.Publish(RepoEvent{RepoID: created.ID, Name: created.Name, Status: "error"})
		} else {
			h.events.Publish(RepoEvent{RepoID: created.ID, Name: created.Name, Status: "ready"})
		}
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "cloning started",
		"repo":    created,
	})
}

// queryInt reads an integer query param with a default value.
func queryInt(c fiber.Ctx, key string, defaultVal int) int {
	v := c.Query(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// GitGraph generates a Mermaid gitGraph diagram for a repo.
func (h *RepoHandler) GitGraph(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	repoID := c.Params("id")
	repo, err := h.store.GetRepoByID(repoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "repo not found"})
	}

	if repo.Status != "ready" || repo.LocalPath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "repo not ready"})
	}

	mermaidStr, authors, err := h.gitVCS.BuildMermaidGitGraph(c.Context(), repo.LocalPath, 150)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"mermaid": mermaidStr, "authors": authors})
}

// StreamEvents streams repo status changes via SSE.
func (h *RepoHandler) StreamEvents(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	ch := h.events.Subscribe()

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer h.events.Unsubscribe(ch)

		// Send heartbeat comment to confirm connection
		fmt.Fprintf(w, ": connected\n\n")
		w.Flush()

		for {
			evt, ok := <-ch
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: repo_status\ndata: %s\n\n", string(data))
			w.Flush()
			slog.Info("SSE repo event", "repo_id", evt.RepoID, "status", evt.Status)
		}
	})
}
