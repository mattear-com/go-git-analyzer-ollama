package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/ai"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/analysis"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/auth"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/vcs"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/handler"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/mcp"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/service"
	"github.com/arturoeanton/go-git-analyzer-ollama/pkg/config"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

func main() {
	// ── Load .env file ───────────────────────────────────────────────────
	_ = godotenv.Load() // silently ignore if .env doesn't exist

	// ── Configuration ────────────────────────────────────────────────────
	cfg := config.Load()

	slog.Info("🚀 Starting CodeLens AI",
		"port", cfg.Port,
		"ollama_embed", cfg.OllamaEmbedURL,
		"ollama_chat", cfg.OllamaChatURL,
		"mcp_enabled", cfg.MCPEnabled,
	)

	// ── Database ─────────────────────────────────────────────────────────
	pgStore, err := store.NewPostgresStore(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()

	vectorStore := store.NewVectorStore(pgStore, cfg.EmbeddingDimension)

	// ── Adapters ─────────────────────────────────────────────────────────
	googleAuth := auth.NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	githubAuth := auth.NewGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURL)

	providers := port.AuthProviderRegistry{
		"google": googleAuth,
		"github": githubAuth,
	}

	ollamaAI := ai.NewOllamaProvider(
		ai.OllamaEndpointConfig{
			BaseURL: cfg.OllamaEmbedURL,
			Model:   cfg.OllamaEmbedModel,
			Token:   cfg.OllamaEmbedToken,
		},
		ai.OllamaEndpointConfig{
			BaseURL: cfg.OllamaChatURL,
			Model:   cfg.OllamaChatModel,
			Token:   cfg.OllamaChatToken,
		},
	)
	gitVCS := vcs.NewGitProvider()

	// ── Analysis Engine (Strategy Pattern) ──────────────────────────────
	engine := port.NewAnalysisEngine(
		analysis.NewArchitectureStrategy(ollamaAI),
		analysis.NewCodeQualityStrategy(ollamaAI),
		analysis.NewFunctionalityStrategy(ollamaAI),
		analysis.NewDevOpsStrategy(ollamaAI),
		analysis.NewSecurityStrategy(ollamaAI),
	)

	// ── Services ─────────────────────────────────────────────────────────
	authService := service.NewAuthService(providers, pgStore, cfg)
	repoService := service.NewRepoService(pgStore, gitVCS, cfg.CloneBasePath)
	analysisService := service.NewAnalysisService(engine)
	ragService := service.NewRAGService(ollamaAI, vectorStore)

	// ── Fiber App ────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      cfg.AppName,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(fiberlogger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{cfg.FrontendURL},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	}))

	// Audit middleware (logs all requests)
	app.Use(middleware.AuditMiddleware(pgStore))

	// ── Public Routes ────────────────────────────────────────────────────
	authHandler := handler.NewAuthHandler(authService, cfg.FrontendURL)
	authHandler.Register(app)

	// Health check
	app.Get("/api/v1/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"app":     cfg.AppName,
			"version": "1.0.0",
		})
	})

	// ── Protected Routes ─────────────────────────────────────────────────
	jwtMiddleware := middleware.JWTMiddleware(middleware.JWTConfig{
		Secret:    cfg.JWTSecret,
		Issuer:    cfg.JWTIssuer,
		ExpiresIn: time.Duration(cfg.JWTExpiration) * time.Hour,
	})

	api := app.Group("/api/v1", jwtMiddleware)

	jobTracker := handler.NewJobTracker()

	repoHandler := handler.NewRepoHandler(repoService, pgStore, gitVCS)
	repoHandler.Register(api)

	analysisHandler := handler.NewAnalysisHandler(analysisService, pgStore, jobTracker, ollamaAI, ragService)
	analysisHandler.Register(api)

	jobsHandler := handler.NewJobsHandler(jobTracker)
	jobsHandler.Register(api)

	reportsHandler := handler.NewReportsHandler(pgStore, vectorStore)
	reportsHandler.Register(api)

	chatHandler := handler.NewChatHandler(ollamaAI, pgStore)
	chatHandler.Register(api)

	ragHandler := handler.NewRAGHandler(ragService)
	ragHandler.Register(api)

	auditHandler := handler.NewAuditHandler(pgStore)
	auditHandler.Register(api)

	streamHandler := handler.NewStreamHandler(pgStore)
	streamHandler.Register(api)

	// Language config endpoint
	api.Put("/repos/:id/language", func(c fiber.Ctx) error {
		var body struct {
			Language string `json:"language"`
		}
		if err := c.Bind().JSON(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		if err := pgStore.SetRepoLanguage(c.Context(), c.Params("id"), body.Language); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	})

	// ── MCP Server (separate port) ───────────────────────────────────────
	if cfg.MCPEnabled {
		mcpServer := mcp.NewServer(ragService, analysisService, cfg.MCPPort)
		go func() {
			if err := mcpServer.Start(); err != nil {
				slog.Error("MCP server failed", "error", err)
			}
		}()
	}

	// ── Start ────────────────────────────────────────────────────────────
	slog.Info("🌐 Fiber listening", "port", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
