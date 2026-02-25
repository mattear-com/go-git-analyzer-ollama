package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AnalysisHandler handles analysis endpoints.
type AnalysisHandler struct {
	analysisService *service.AnalysisService
	store           *store.PostgresStore
	tracker         *JobTracker
	ai              port.AIProvider
	ragService      *service.RAGService
}

// NewAnalysisHandler creates a new analysis handler.
func NewAnalysisHandler(analysisService *service.AnalysisService, pgStore *store.PostgresStore, tracker *JobTracker, ai port.AIProvider, ragSvc *service.RAGService) *AnalysisHandler {
	return &AnalysisHandler{
		analysisService: analysisService,
		store:           pgStore,
		tracker:         tracker,
		ai:              ai,
		ragService:      ragSvc,
	}
}

// Register sets up analysis routes.
func (h *AnalysisHandler) Register(router fiber.Router) {
	analysis := router.Group("/analysis")
	analysis.Get("/strategies", h.ListStrategies)
	analysis.Post("/run", h.RunAnalysis)
}

// ListStrategies returns available analysis strategies.
func (h *AnalysisHandler) ListStrategies(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"strategies": h.analysisService.ListStrategies(),
	})
}

// RunAnalysis accepts a job and returns 202 immediately. Runs all strategies in background.
func (h *AnalysisHandler) RunAnalysis(c fiber.Ctx) error {
	uc := middleware.GetUserContext(c)
	if uc == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var body struct {
		RepoID string `json:"repo_id"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Build request with actual repo data
	req, repo, err := h.buildAnalysisRequest(body.RepoID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	strategies := h.analysisService.ListStrategies()
	jobID := uuid.New().String()

	h.tracker.CreateJob(jobID, body.RepoID, len(strategies))

	// Run analysis in background — NO HTTP connection held
	go h.runAnalysisJob(jobID, body.RepoID, req, strategies, repo.ReportLanguage)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id":     jobID,
		"strategies": strategies,
		"message":    "analysis started",
	})
}

// runAnalysisJob runs all strategies sequentially in background.
func (h *AnalysisHandler) runAnalysisJob(jobID, repoID string, req port.AnalysisRequest, strategies []string, lang string) {
	ctx := context.Background()

	// Index code chunks for RAG embeddings in parallel (best-effort, non-blocking)
	if h.ragService != nil {
		repo, repoErr := h.store.GetRepoByID(repoID)
		if repoErr == nil && repo.LocalPath != "" {
			// Create a snapshot record so embeddings have a valid FK
			snap, snapErr := h.store.CreateSnapshot(ctx, &domain.Snapshot{
				RepoID:     repoID,
				CommitHash: "analysis-" + jobID[:8],
				Branch:     "HEAD",
				Message:    "RAG indexing for analysis",
				Author:     "system",
				FileCount:  0,
				Status:     domain.SnapshotStatusPending,
			})
			if snapErr != nil {
				slog.Error("create snapshot for RAG failed", "error", snapErr)
			} else {
				localPath := repo.LocalPath
				snapshotID := snap.ID
				// Run indexing in parallel so analysis starts immediately
				go func() {
					indexCtx := context.Background()
					// Blacklist: skip binary/non-useful files; include everything else
					skipExts := map[string]bool{
						// Images
						".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".svg": true, ".webp": true, ".tiff": true,
						// Video/Audio
						".mp4": true, ".avi": true, ".mov": true, ".mp3": true, ".wav": true, ".flac": true, ".ogg": true, ".webm": true,
						// Fonts
						".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
						// Archives
						".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true, ".rar": true, ".jar": true, ".war": true,
						// Compiled/Binary
						".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true, ".class": true, ".pyc": true, ".wasm": true,
						// Lock files & large generated
						".lock": true,
						// Data files
						".sqlite": true, ".db": true, ".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true,
						// Maps
						".map": true,
					}
					skipFiles := map[string]bool{
						"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
						"go.sum": true, "Cargo.lock": true, "Gemfile.lock": true,
						"composer.lock": true, "poetry.lock": true, "Pipfile.lock": true,
					}
					files := make(map[string]string)
					_ = filepath.Walk(localPath, func(path string, info os.FileInfo, walkErr error) error {
						if walkErr != nil || info.IsDir() {
							base := filepath.Base(path)
							if info != nil && info.IsDir() && (strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" || base == "__pycache__" || base == "dist" || base == "build" || base == "target") {
								return filepath.SkipDir
							}
							return nil
						}
						baseName := filepath.Base(path)
						if skipFiles[baseName] {
							return nil
						}
						ext := strings.ToLower(filepath.Ext(path))
						if skipExts[ext] {
							return nil
						}
						if info.Size() > 50000 {
							return nil
						}
						relPath, _ := filepath.Rel(localPath, path)
						content, readErr := os.ReadFile(path)
						if readErr == nil {
							files[relPath] = string(content)
						}
						return nil
					})
					if len(files) > 0 {
						slog.Info("indexing code for RAG (parallel)", "repo_id", repoID, "snapshot_id", snapshotID, "files", len(files))
						if err := h.ragService.IndexChunks(indexCtx, repoID, snapshotID, files); err != nil {
							slog.Error("RAG indexing failed", "error", err)
						} else {
							slog.Info("RAG indexing complete", "repo_id", repoID, "files", len(files))
						}
					}
				}()
			}
		}
	}

	for i, strategy := range strategies {
		h.tracker.UpdateJob(jobID, strategy, i, "running")
		slog.Info("running strategy", "job_id", jobID, "strategy", strategy, "progress", fmt.Sprintf("%d/%d", i+1, len(strategies)))

		var result *port.AnalysisResult
		var err error
		maxRetries := 2
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				slog.Warn("retrying strategy", "strategy", strategy, "attempt", attempt+1, "max", maxRetries+1)
				time.Sleep(5 * time.Second)
			}
			result, err = h.analysisService.RunStrategy(ctx, strategy, req)
			if err == nil {
				break
			}
			slog.Error("strategy attempt failed", "strategy", strategy, "attempt", attempt+1, "error", err)
		}

		if err != nil {
			slog.Error("strategy failed after retries", "strategy", strategy, "error", err)
			// Save a failure report so the user knows
			failSummary := fmt.Sprintf("## ⚠️ Analysis Failed\n\nThe **%s** strategy could not be completed after %d attempts.\n\n**Error:** `%s`\n\nYou can re-run the analysis to try again.",
				strategy, maxRetries+1, err.Error())
			_ = h.store.SaveAnalysisResultFull(ctx, repoID, strategy, failSummary, "{}", 0, "")
			h.tracker.UpdateJob(jobID, strategy, i+1, "running")
			continue
		}

		// Save English result
		summary := result.Summary
		detailsJSON, _ := json.Marshal(result.Details)
		translated := ""

		// Translate if needed
		if lang != "" && lang != "en" {
			translated = h.translateReport(ctx, summary, lang)
		}

		if saveErr := h.store.SaveAnalysisResultFull(ctx, repoID, strategy, summary, string(detailsJSON), result.Score, translated); saveErr != nil {
			slog.Error("failed to save analysis result", "error", saveErr)
		}

		h.tracker.UpdateJob(jobID, strategy, i+1, "running")
	}

	h.tracker.UpdateJob(jobID, "", len(strategies), "complete")
	slog.Info("analysis job complete", "job_id", jobID)
}

// translateReport uses Ollama to translate a markdown report.
func (h *AnalysisHandler) translateReport(ctx context.Context, markdown string, targetLang string) string {
	langNames := map[string]string{
		"es": "Spanish", "pt": "Portuguese", "fr": "French", "de": "German",
		"it": "Italian", "ja": "Japanese", "ko": "Korean", "zh": "Chinese",
	}

	langName := langNames[targetLang]
	if langName == "" {
		langName = targetLang
	}

	systemPrompt := fmt.Sprintf(`Translate the following technical report to %s. 
Keep all Markdown formatting, Mermaid diagrams, code blocks, and technical terms intact.
Only translate the natural language text. Do NOT add any commentary or explanation.`, langName)

	translated, err := h.ai.Chat(ctx, systemPrompt, markdown, nil)
	if err != nil {
		slog.Error("translation failed", "lang", targetLang, "error", err)
		return ""
	}
	return translated
}

// buildAnalysisRequest reads the cloned repo from disk.
func (h *AnalysisHandler) buildAnalysisRequest(repoID string) (port.AnalysisRequest, *repoInfo, error) {
	repo, err := h.store.GetRepoByID(repoID)
	if err != nil {
		return port.AnalysisRequest{}, nil, fmt.Errorf("repo not found: %w", err)
	}

	if repo.LocalPath == "" || repo.Status != "ready" {
		return port.AnalysisRequest{}, nil, fmt.Errorf("repo not cloned or not ready (status: %s)", repo.Status)
	}

	var fileTree []string
	var chunks []string

	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".java": true, ".rs": true, ".rb": true, ".swift": true, ".kt": true, ".c": true,
		".cpp": true, ".h": true, ".cs": true, ".php": true, ".sh": true,
		".yaml": true, ".yml": true, ".toml": true, ".json": true,
		".sql": true, ".proto": true, ".tf": true, ".md": true,
	}

	configFiles := map[string]bool{
		"Dockerfile": true, "docker-compose.yml": true, "docker-compose.yaml": true,
		"Makefile": true, "go.mod": true, "package.json": true, "requirements.txt": true,
		"README.md": true, ".gitignore": true,
	}

	maxChunks := 30
	maxFileSize := 8000
	totalChars := 0
	maxTotalChars := 60000

	_ = filepath.Walk(repo.LocalPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(repo.LocalPath, path)
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" ||
				base == "__pycache__" || base == "dist" || base == "build" || base == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		fileTree = append(fileTree, relPath)

		ext := strings.ToLower(filepath.Ext(path))
		baseName := filepath.Base(path)
		if !codeExts[ext] && !configFiles[baseName] {
			return nil
		}
		if info.Size() > int64(maxFileSize) || len(chunks) >= maxChunks || totalChars >= maxTotalChars {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		chunk := fmt.Sprintf("=== %s ===\n%s", relPath, string(content))
		chunks = append(chunks, chunk)
		totalChars += len(chunk)
		return nil
	})

	slog.Info("analysis request built", "repo", repo.Name, "files", len(fileTree), "chunks", len(chunks))

	return port.AnalysisRequest{
		RepoID:   repoID,
		RepoName: repo.Name,
		Chunks:   chunks,
		FileTree: fileTree,
	}, &repoInfo{ReportLanguage: repo.ReportLanguage}, nil
}

type repoInfo struct {
	ReportLanguage string
}
