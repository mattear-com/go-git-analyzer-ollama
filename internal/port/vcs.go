package port

import (
	"context"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
)

// VCSProvider abstracts version control system operations.
// Implementations handle cloning, log retrieval, and diff generation.
type VCSProvider interface {
	// Clone clones a repository from url into dest directory.
	Clone(ctx context.Context, url string, dest string) error

	// Pull fetches the latest changes for an existing local repository.
	Pull(ctx context.Context, repoPath string) error

	// Log returns the commit history of a repository.
	Log(ctx context.Context, repoPath string, limit int) ([]domain.CommitInfo, error)

	// Diff returns the unified diff between two commits.
	Diff(ctx context.Context, repoPath, fromHash, toHash string) (string, error)

	// ListFiles returns all file paths in the repository at a given commit.
	ListFiles(ctx context.Context, repoPath string, commitHash string) ([]string, error)

	// ReadFile reads a file's content at a specific commit hash.
	ReadFile(ctx context.Context, repoPath string, commitHash string, filePath string) ([]byte, error)
}
