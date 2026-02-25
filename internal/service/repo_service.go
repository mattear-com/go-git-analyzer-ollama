package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

// RepoService manages repository lifecycle — cloning, indexing, and listing.
type RepoService struct {
	store    *store.PostgresStore
	vcs      port.VCSProvider
	basePath string
}

// NewRepoService creates a new repository service.
func NewRepoService(s *store.PostgresStore, vcs port.VCSProvider, basePath string) *RepoService {
	return &RepoService{store: s, vcs: vcs, basePath: basePath}
}

// AddRepo clones and registers a new repository.
func (s *RepoService) AddRepo(ctx context.Context, userID, name, url string) (*domain.Repo, error) {
	localPath := filepath.Join(s.basePath, userID, name)

	repo := &domain.Repo{
		UserID:        userID,
		Name:          name,
		URL:           url,
		DefaultBranch: "main",
		LocalPath:     localPath,
		Status:        domain.RepoStatusCloning,
	}

	repo, err := s.store.CreateRepo(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("create repo: %w", err)
	}

	// Clone asynchronously
	go func() {
		slog.Info("cloning repository", "repo_id", repo.ID, "url", url)
		if err := s.vcs.Clone(context.Background(), url, localPath); err != nil {
			slog.Error("clone failed", "repo_id", repo.ID, "error", err)
			_ = s.store.UpdateRepoStatus(context.Background(), repo.ID, domain.RepoStatusError, localPath)
			return
		}
		_ = s.store.UpdateRepoStatus(context.Background(), repo.ID, domain.RepoStatusReady, localPath)
		slog.Info("clone complete", "repo_id", repo.ID)
	}()

	return repo, nil
}

// ListRepos returns all repos for a user.
func (s *RepoService) ListRepos(ctx context.Context, userID string) ([]domain.Repo, error) {
	return s.store.ListReposByUser(ctx, userID)
}

// CloneRepo clones a repository from its URL to the local filesystem.
func (s *RepoService) CloneRepo(repo *domain.Repo) error {
	localPath := filepath.Join(s.basePath, repo.UserID, repo.Name)

	slog.Info("cloning repository", "repo_id", repo.ID, "url", repo.URL)
	if err := s.vcs.Clone(context.Background(), repo.URL, localPath); err != nil {
		slog.Error("clone failed", "repo_id", repo.ID, "error", err)
		_ = s.store.UpdateRepoStatus(context.Background(), repo.ID, "error", "")
		return fmt.Errorf("clone repo: %w", err)
	}

	_ = s.store.UpdateRepoStatus(context.Background(), repo.ID, "ready", localPath)
	slog.Info("clone complete", "repo_id", repo.ID)
	return nil
}

// CloneRepoWithURL clones using a specific URL (e.g. with embedded auth token).
func (s *RepoService) CloneRepoWithURL(repo *domain.Repo, authURL string) error {
	localPath := filepath.Join(s.basePath, repo.UserID, repo.Name)

	slog.Info("cloning repository", "repo_id", repo.ID, "name", repo.Name)
	if err := s.vcs.Clone(context.Background(), authURL, localPath); err != nil {
		slog.Error("clone failed", "repo_id", repo.ID, "error", err)
		_ = s.store.UpdateRepoStatus(context.Background(), repo.ID, "error", "")
		return fmt.Errorf("clone repo: %w", err)
	}

	_ = s.store.UpdateRepoStatus(context.Background(), repo.ID, "ready", localPath)
	slog.Info("clone complete", "repo_id", repo.ID)
	return nil
}
