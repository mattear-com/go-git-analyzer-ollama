package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
	"github.com/gofiber/fiber/v3"
)

// JWTConfig holds JWT middleware configuration.
type JWTConfig struct {
	Secret    string
	Issuer    string
	ExpiresIn time.Duration
}

// JWTMiddleware creates a Fiber middleware that validates JWT tokens
// and injects a UserContext into the request context.
func JWTMiddleware(cfg JWTConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		var token string

		// Try Authorization header first
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				token = parts[1]
			}
		}

		// Fallback: ?token= query param (for SSE/EventSource which can't set headers)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization",
			})
		}

		claims, err := validateJWT(token, cfg.Secret, cfg.Issuer)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Inject UserContext into Fiber locals
		c.Locals("user", &domain.UserContext{
			UserID: claims.Subject,
			Email:  claims.Email,
			Name:   claims.Name,
			Role:   claims.Role,
		})

		return c.Next()
	}
}

// GetUserContext extracts the UserContext from Fiber locals.
func GetUserContext(c fiber.Ctx) *domain.UserContext {
	u, ok := c.Locals("user").(*domain.UserContext)
	if !ok {
		return nil
	}
	return u
}

// --- JWT Claims & Helpers ---

// Claims represents the JWT payload.
type Claims struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Issuer    string `json:"iss"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// GenerateJWT creates a new signed JWT for the given user.
func GenerateJWT(user *domain.User, cfg JWTConfig) (string, error) {
	now := time.Now()
	claims := Claims{
		Subject:   user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		Issuer:    cfg.Issuer,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(cfg.ExpiresIn).Unix(),
	}

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	signature := signHS256(signingInput, cfg.Secret)

	return signingInput + "." + signature, nil
}

func validateJWT(tokenStr, secret, expectedIssuer string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig := signHS256(signingInput, secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding")
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate expiration
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	// Validate issuer
	if claims.Issuer != expectedIssuer {
		return nil, fmt.Errorf("invalid token issuer")
	}

	return &claims, nil
}

func signHS256(input, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
