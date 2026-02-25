package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/middleware"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
	"github.com/arturoeanton/go-git-analyzer-ollama/pkg/config"
)

// AuthService handles the authentication flow.
type AuthService struct {
	providers port.AuthProviderRegistry
	store     *store.PostgresStore
	jwtCfg    middleware.JWTConfig
}

// NewAuthService creates a new authentication service.
func NewAuthService(providers port.AuthProviderRegistry, store *store.PostgresStore, cfg *config.Config) *AuthService {
	return &AuthService{
		providers: providers,
		store:     store,
		jwtCfg: middleware.JWTConfig{
			Secret:    cfg.JWTSecret,
			Issuer:    cfg.JWTIssuer,
			ExpiresIn: time.Duration(cfg.JWTExpiration) * time.Hour,
		},
	}
}

// GetAuthURL returns the OAuth2 authorization URL for the given provider.
func (s *AuthService) GetAuthURL(providerName, state string) (string, error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerName)
	}
	return provider.AuthURL(state), nil
}

// HandleCallback processes the OAuth2 callback, exchanges code, upserts user, and returns a JWT.
func (s *AuthService) HandleCallback(ctx context.Context, providerName, code string) (string, *domain.User, error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return "", nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	// Exchange authorization code for tokens
	tokens, err := provider.ExchangeCode(ctx, code)
	if err != nil {
		return "", nil, fmt.Errorf("exchange code: %w", err)
	}

	// Fetch user profile
	profile, err := provider.GetUserProfile(ctx, tokens.AccessToken)
	if err != nil {
		return "", nil, fmt.Errorf("get profile: %w", err)
	}

	// Store the OAuth access token for later API calls (e.g. GitHub repos)
	profile.AccessToken = tokens.AccessToken

	// Upsert user in database
	user, err := s.store.UpsertUser(ctx, profile)
	if err != nil {
		return "", nil, fmt.Errorf("upsert user: %w", err)
	}

	// Generate JWT
	jwt, err := middleware.GenerateJWT(user, s.jwtCfg)
	if err != nil {
		return "", nil, fmt.Errorf("generate jwt: %w", err)
	}

	slog.Info("user authenticated", "user_id", user.ID, "provider", providerName)
	return jwt, user, nil
}
