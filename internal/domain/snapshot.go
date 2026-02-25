package domain

import "time"

// Snapshot represents an immutable point-in-time capture of a repository at a specific commit.
type Snapshot struct {
	ID         string    `json:"id"          db:"id"`
	RepoID     string    `json:"repo_id"     db:"repo_id"`
	CommitHash string    `json:"commit_hash" db:"commit_hash"`
	Branch     string    `json:"branch"      db:"branch"`
	Message    string    `json:"message"     db:"message"`
	Author     string    `json:"author"      db:"author"`
	FileCount  int       `json:"file_count"  db:"file_count"`
	Status     string    `json:"status"      db:"status"` // pending, vectorized, analyzed
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

// CommitInfo is a lightweight representation of a git commit for log output.
type CommitInfo struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Files     int       `json:"files_changed"`
}

// Snapshot status constants.
const (
	SnapshotStatusPending    = "pending"
	SnapshotStatusVectorized = "vectorized"
	SnapshotStatusAnalyzed   = "analyzed"
)
