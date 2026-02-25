package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port    string
	AppName string

	// Database
	DatabaseURL string

	// OAuth2 — Google
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// OAuth2 — GitHub
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string

	// JWT
	JWTSecret     string
	JWTIssuer     string
	JWTExpiration int // hours

	// Ollama — Embed endpoint
	OllamaEmbedURL   string
	OllamaEmbedModel string
	OllamaEmbedToken string // Bearer token for Ollama Cloud (empty = local)

	// Ollama — Chat/Analysis endpoint
	OllamaChatURL   string
	OllamaChatModel string
	OllamaChatToken string // Bearer token for Ollama Cloud (empty = local)

	EmbeddingDimension int

	// Repos
	CloneBasePath string

	// MCP
	MCPEnabled bool
	MCPPort    string

	// Frontend
	FrontendURL string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:    envOrDefault("PORT", "3001"),
		AppName: envOrDefault("APP_NAME", "CodeLens AI"),

		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://codelens:codelens@localhost:5432/codelens?sslmode=disable"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  envOrDefault("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/callback"),

		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubRedirectURL:  envOrDefault("GITHUB_REDIRECT_URL", "http://localhost:8080/auth/callback"),

		JWTSecret:     envOrDefault("JWT_SECRET", "change-me-in-production"),
		JWTIssuer:     envOrDefault("JWT_ISSUER", "codelens-ai"),
		JWTExpiration: envOrDefaultInt("JWT_EXPIRATION_HOURS", 24),

		OllamaEmbedURL:   envOrDefault("OLLAMA_EMBED_URL", envOrDefault("OLLAMA_BASE_URL", "http://localhost:11434")),
		OllamaEmbedModel: envOrDefault("OLLAMA_EMBED_MODEL", "bge-m3"),
		OllamaEmbedToken: os.Getenv("OLLAMA_EMBED_TOKEN"),

		OllamaChatURL:   envOrDefault("OLLAMA_CHAT_URL", envOrDefault("OLLAMA_BASE_URL", "http://localhost:11434")),
		OllamaChatModel: envOrDefault("OLLAMA_CHAT_MODEL", "qwen3"),
		OllamaChatToken: os.Getenv("OLLAMA_CHAT_TOKEN"),

		EmbeddingDimension: envOrDefaultInt("EMBEDDING_DIMENSION", 1024),

		CloneBasePath: envOrDefault("CLONE_BASE_PATH", "/tmp/codelens-repos"),

		MCPEnabled: envOrDefaultBool("MCP_ENABLED", true),
		MCPPort:    envOrDefault("MCP_PORT", "3002"),

		FrontendURL: envOrDefault("FRONTEND_URL", "http://localhost:3000"),
	}
}

// DSN returns a formatted connection string for logging (password masked).
func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://***@***/codelens (from DATABASE_URL)")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func envOrDefaultBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return fallback
}
