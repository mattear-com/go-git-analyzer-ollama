package port

import (
	"context"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
)

// AuthProvider abstracts the OAuth2 identity provider.
// Implementations handle token exchange and user profile retrieval
// for a specific provider (Google, GitHub, etc.).
type AuthProvider interface {
	// ProviderName returns the name of this provider (e.g. "google", "github").
	ProviderName() string

	// AuthURL returns the full OAuth2 authorization URL for redirecting the user.
	AuthURL(state string) string

	// ExchangeCode exchanges an authorization code for an access/refresh token pair.
	ExchangeCode(ctx context.Context, code string) (*domain.TokenPair, error)

	// GetUserProfile fetches the authenticated user's profile from the provider.
	GetUserProfile(ctx context.Context, accessToken string) (*domain.User, error)
}

// AuthProviderRegistry holds multiple AuthProvider implementations keyed by name.
type AuthProviderRegistry map[string]AuthProvider
