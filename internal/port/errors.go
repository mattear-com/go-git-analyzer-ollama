package port

import "errors"

// Sentinel errors used across ports.
var (
	ErrStrategyNotFound = errors.New("analysis strategy not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenInvalid     = errors.New("token invalid")
	ErrUserNotFound     = errors.New("user not found")
	ErrRepoNotFound     = errors.New("repository not found")
	ErrSnapshotNotFound = errors.New("snapshot not found")
)
