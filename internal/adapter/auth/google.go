package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
)

const (
	googleAuthURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL   = "https://oauth2.googleapis.com/token"
	googleProfileURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GoogleProvider implements port.AuthProvider for Google OAuth2.
type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

// NewGoogleProvider creates a new Google OAuth2 provider.
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		httpClient:   &http.Client{},
	}
}

// ProviderName returns "google".
func (g *GoogleProvider) ProviderName() string {
	return "google"
}

// AuthURL returns the Google OAuth2 consent screen URL.
func (g *GoogleProvider) AuthURL(state string) string {
	params := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {g.redirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	return fmt.Sprintf("%s?%s", googleAuthURL, params.Encode())
}

// ExchangeCode exchanges an authorization code for tokens.
func (g *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*domain.TokenPair, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {g.redirectURL},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("google: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google: token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp domain.TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("google: decode token response: %w", err)
	}

	return &tokenResp, nil
}

// GetUserProfile fetches the Google user profile using an access token.
func (g *GoogleProvider) GetUserProfile(ctx context.Context, accessToken string) (*domain.User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleProfileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("google: create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google: profile fetch failed (%d): %s", resp.StatusCode, string(body))
	}

	var profile struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("google: decode profile: %w", err)
	}

	return &domain.User{
		Email:      profile.Email,
		Name:       profile.Name,
		AvatarURL:  profile.Picture,
		Provider:   "google",
		ProviderID: profile.ID,
	}, nil
}
