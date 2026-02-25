package domain

import "time"

// Repo represents a tracked Git repository.
type Repo struct {
	ID             string    `json:"id"           db:"id"`
	UserID         string    `json:"user_id"      db:"user_id"`
	Name           string    `json:"name"         db:"name"`
	URL            string    `json:"url"          db:"url"`
	DefaultBranch  string    `json:"default_branch" db:"default_branch"`
	LocalPath      string    `json:"-"            db:"local_path"`
	Status         string    `json:"status"       db:"status"` // cloning, ready, error
	ReportLanguage string    `json:"report_language" db:"report_language"`
	CreatedAt      time.Time `json:"created_at"   db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"   db:"updated_at"`
}

// RepoStatus constants.
const (
	RepoStatusCloning = "cloning"
	RepoStatusReady   = "ready"
	RepoStatusError   = "error"
)
