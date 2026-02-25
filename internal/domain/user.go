package domain

import "time"

// User represents an authenticated user in the system.
type User struct {
	ID          string    `json:"id"          db:"id"`
	Email       string    `json:"email"       db:"email"`
	Name        string    `json:"name"        db:"name"`
	AvatarURL   string    `json:"avatar_url"  db:"avatar_url"`
	Provider    string    `json:"provider"    db:"provider"`
	ProviderID  string    `json:"provider_id" db:"provider_id"`
	Role        string    `json:"role"        db:"role"`
	AccessToken string    `json:"-"           db:"access_token"` // never serialized to JSON
	CreatedAt   time.Time `json:"created_at"  db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"  db:"updated_at"`
}

// TokenPair holds the OAuth2 tokens returned after code exchange.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// UserContext is the authenticated user context injected into request handlers.
type UserContext struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}
