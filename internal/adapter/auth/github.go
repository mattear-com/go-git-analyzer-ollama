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
	githubAuthURL    = "https://github.com/login/oauth/authorize"
	githubTokenURL   = "https://github.com/login/oauth/access_token"
	githubProfileURL = "https://api.github.com/user"
	githubEmailsURL  = "https://api.github.com/user/emails"
)

// GitHubProvider implements port.AuthProvider for GitHub OAuth.
type GitHubProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

// NewGitHubProvider creates a new GitHub OAuth provider.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		httpClient:   &http.Client{},
	}
}

// ProviderName returns "github".
func (g *GitHubProvider) ProviderName() string {
	return "github"
}

// AuthURL returns the GitHub OAuth consent screen URL.
func (g *GitHubProvider) AuthURL(state string) string {
	params := url.Values{
		"client_id":    {g.clientID},
		"redirect_uri": {g.redirectURL},
		"scope":        {"user:email read:user repo read:org"},
		"state":        {state},
	}
	return fmt.Sprintf("%s?%s", githubAuthURL, params.Encode())
}

// ExchangeCode exchanges an authorization code for tokens.
func (g *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*domain.TokenPair, error) {
	data := url.Values{
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"code":          {code},
		"redirect_uri":  {g.redirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("github: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("github: decode token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &domain.TokenPair{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
	}, nil
}

// GetUserProfile fetches the GitHub user profile using an access token.
func (g *GitHubProvider) GetUserProfile(ctx context.Context, accessToken string) (*domain.User, error) {
	// Fetch profile
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubProfileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: profile fetch failed (%d): %s", resp.StatusCode, string(body))
	}

	var profile struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("github: decode profile: %w", err)
	}

	// If email is private, fetch from /user/emails
	email := profile.Email
	if email == "" {
		email, _ = g.fetchPrimaryEmail(ctx, accessToken)
	}

	name := profile.Name
	if name == "" {
		name = profile.Login
	}

	return &domain.User{
		Email:      email,
		Name:       name,
		AvatarURL:  profile.AvatarURL,
		Provider:   "github",
		ProviderID: fmt.Sprintf("%d", profile.ID),
	}, nil
}

// fetchPrimaryEmail gets the user's primary verified email from /user/emails.
func (g *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubEmailsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", fmt.Errorf("no email found")
}
