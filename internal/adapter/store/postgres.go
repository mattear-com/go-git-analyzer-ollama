package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
)

// PostgresStore handles all relational database operations.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore opens a connection and returns a store instance.
func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// Close closes the database connection.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use in transactions.
func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

// --- Users ---

// UpsertUser inserts or updates a user by provider + provider_id.
func (s *PostgresStore) UpsertUser(ctx context.Context, u *domain.User) (*domain.User, error) {
	query := `
		INSERT INTO users (email, name, avatar_url, provider, provider_id, role, access_token)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider, provider_id) DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			access_token = EXCLUDED.access_token,
			updated_at = NOW()
		RETURNING id, email, name, avatar_url, provider, provider_id, role, created_at, updated_at`

	row := s.db.QueryRowContext(ctx, query,
		u.Email, u.Name, u.AvatarURL, u.Provider, u.ProviderID, "user", u.AccessToken,
	)

	var user domain.User
	err := row.Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.Role,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &user, nil
}

// GetUserByID retrieves a user by ID.
func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	query := `SELECT id, email, name, avatar_url, provider, provider_id, role, access_token, created_at, updated_at
	          FROM users WHERE id = $1`

	var user domain.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.Provider, &user.ProviderID, &user.Role, &user.AccessToken,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

// --- Repos ---

// CreateRepo inserts a new repository record.
func (s *PostgresStore) CreateRepo(ctx context.Context, r *domain.Repo) (*domain.Repo, error) {
	query := `INSERT INTO repos (user_id, name, url, default_branch, local_path, status)
	          VALUES ($1, $2, $3, $4, $5, $6)
	          RETURNING id, user_id, name, url, default_branch, local_path, status, report_language, created_at, updated_at`

	var repo domain.Repo
	err := s.db.QueryRowContext(ctx, query,
		r.UserID, r.Name, r.URL, r.DefaultBranch, r.LocalPath, r.Status,
	).Scan(
		&repo.ID, &repo.UserID, &repo.Name, &repo.URL, &repo.DefaultBranch,
		&repo.LocalPath, &repo.Status, &repo.ReportLanguage, &repo.CreatedAt, &repo.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create repo: %w", err)
	}
	return &repo, nil
}

// GetRepoByID returns a repo by its ID.
func (s *PostgresStore) GetRepoByID(repoID string) (*domain.Repo, error) {
	query := `SELECT id, user_id, name, url, default_branch, local_path, status, report_language, created_at, updated_at
	          FROM repos WHERE id = $1`

	var r domain.Repo
	err := s.db.QueryRow(query, repoID).Scan(
		&r.ID, &r.UserID, &r.Name, &r.URL, &r.DefaultBranch,
		&r.LocalPath, &r.Status, &r.ReportLanguage, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get repo: %w", err)
	}
	return &r, nil
}

// ListReposByUser returns all repos for a user.
func (s *PostgresStore) ListReposByUser(ctx context.Context, userID string) ([]domain.Repo, error) {
	query := `SELECT id, user_id, name, url, default_branch, local_path, status, report_language, created_at, updated_at
	          FROM repos WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()

	var repos []domain.Repo
	for rows.Next() {
		var r domain.Repo
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Name, &r.URL, &r.DefaultBranch,
			&r.LocalPath, &r.Status, &r.ReportLanguage, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, nil
}

// UpdateRepoStatus updates the status and local_path of a repo.
func (s *PostgresStore) UpdateRepoStatus(ctx context.Context, id, status, localPath string) error {
	query := `UPDATE repos SET status = $1, local_path = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, status, localPath, id)
	return err
}

// --- Snapshots ---

// CreateSnapshot creates a new snapshot record.
func (s *PostgresStore) CreateSnapshot(ctx context.Context, snap *domain.Snapshot) (*domain.Snapshot, error) {
	query := `INSERT INTO snapshots (repo_id, commit_hash, branch, message, author, file_count, status)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)
	          ON CONFLICT (repo_id, commit_hash) DO UPDATE SET status = snapshots.status
	          RETURNING id, repo_id, commit_hash, branch, message, author, file_count, status, created_at`

	var result domain.Snapshot
	err := s.db.QueryRowContext(ctx, query,
		snap.RepoID, snap.CommitHash, snap.Branch, snap.Message, snap.Author, snap.FileCount, snap.Status,
	).Scan(
		&result.ID, &result.RepoID, &result.CommitHash, &result.Branch,
		&result.Message, &result.Author, &result.FileCount, &result.Status, &result.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}
	return &result, nil
}

// --- Audit Logs ---

// WriteAudit implements middleware.AuditWriter.
func (s *PostgresStore) WriteAudit(userID, action, resource, resourceID, details, ip, userAgent string) error {
	query := `INSERT INTO audit_logs (user_id, action, resource, resource_id, details, ip, user_agent)
	          VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`
	_, err := s.db.ExecContext(context.Background(), query,
		userID, action, resource, resourceID, details, ip, userAgent,
	)
	return err
}

// ListAuditLogs returns recent audit logs with optional filters.
func (s *PostgresStore) ListAuditLogs(ctx context.Context, limit int, action string) ([]domain.AuditLog, error) {
	query := `SELECT id, user_id, action, resource, resource_id, details, ip, user_agent, created_at
	          FROM audit_logs`
	args := []interface{}{}
	argIdx := 1

	if action != "" {
		query += fmt.Sprintf(" WHERE action = $%d", argIdx)
		args = append(args, action)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(
			&l.ID, &l.UserID, &l.Action, &l.Resource, &l.ResourceID,
			&l.Details, &l.IP, &l.UserAgent, &l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// --- Analysis Results ---

// AnalysisResultRow represents a stored analysis result.
type AnalysisResultRow struct {
	ID                string    `json:"id"`
	RepoID            string    `json:"repo_id"`
	Strategy          string    `json:"strategy"`
	Summary           string    `json:"summary"`
	SummaryTranslated string    `json:"summary_translated"`
	Details           string    `json:"details"`
	Score             float64   `json:"score"`
	CreatedAt         time.Time `json:"created_at"`
}

// SaveAnalysisResult persists an analysis result (English only).
func (s *PostgresStore) SaveAnalysisResult(ctx context.Context, repoID, strategy, summary, details string, score float64) error {
	return s.SaveAnalysisResultFull(ctx, repoID, strategy, summary, details, score, "")
}

// SaveAnalysisResultFull persists an analysis result with optional translation.
func (s *PostgresStore) SaveAnalysisResultFull(ctx context.Context, repoID, strategy, summary, details string, score float64, translated string) error {
	if !json.Valid([]byte(details)) {
		wrapped, _ := json.Marshal(map[string]string{"raw": details})
		details = string(wrapped)
	}
	if details == "" {
		details = "{}"
	}

	query := `INSERT INTO analysis_results (repo_id, strategy, summary, details, score, summary_translated)
	          VALUES ($1, $2, $3, $4::jsonb, $5, $6)`
	_, err := s.db.ExecContext(ctx, query, repoID, strategy, summary, details, score, translated)
	return err
}

// SetRepoLanguage sets the report language for a repo.
func (s *PostgresStore) SetRepoLanguage(ctx context.Context, repoID, lang string) error {
	query := `UPDATE repos SET report_language = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, lang, repoID)
	return err
}

// ListAnalysisResults returns analysis results for a repo, newest first.
func (s *PostgresStore) ListAnalysisResults(ctx context.Context, repoID string) ([]AnalysisResultRow, error) {
	query := `SELECT id, repo_id, strategy, summary, COALESCE(summary_translated, ''), COALESCE(details::text, '{}'), score, created_at
	          FROM analysis_results WHERE repo_id = $1 ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("list analysis results: %w", err)
	}
	defer rows.Close()

	var results []AnalysisResultRow
	for rows.Next() {
		var r AnalysisResultRow
		if err := rows.Scan(&r.ID, &r.RepoID, &r.Strategy, &r.Summary, &r.SummaryTranslated, &r.Details, &r.Score, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan analysis result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

// ListAllAnalysisResults returns all analysis results for a user's repos, newest first.
func (s *PostgresStore) ListAllAnalysisResults(ctx context.Context, userID string) ([]AnalysisResultRow, error) {
	query := `SELECT ar.id, ar.repo_id, ar.strategy, ar.summary, COALESCE(ar.summary_translated, ''), COALESCE(ar.details::text, '{}'), ar.score, ar.created_at
	          FROM analysis_results ar
	          JOIN repos r ON r.id = ar.repo_id
	          WHERE r.user_id = $1
	          ORDER BY ar.created_at DESC
	          LIMIT 200`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list all analysis results: %w", err)
	}
	defer rows.Close()

	var results []AnalysisResultRow
	for rows.Next() {
		var r AnalysisResultRow
		if err := rows.Scan(&r.ID, &r.RepoID, &r.Strategy, &r.Summary, &r.SummaryTranslated, &r.Details, &r.Score, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan analysis result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

// --- Search ---

// SearchReposByUser searches repos by name or url using ILIKE, scoped to a user.
func (s *PostgresStore) SearchReposByUser(ctx context.Context, userID, query string) ([]domain.Repo, error) {
	pattern := "%" + query + "%"
	sqlQuery := `SELECT id, user_id, name, url, default_branch, local_path, status, report_language, created_at, updated_at
	             FROM repos
	             WHERE user_id = $1 AND (name ILIKE $2 OR url ILIKE $2)
	             ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, sqlQuery, userID, pattern)
	if err != nil {
		return nil, fmt.Errorf("search repos: %w", err)
	}
	defer rows.Close()

	var repos []domain.Repo
	for rows.Next() {
		var r domain.Repo
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Name, &r.URL, &r.DefaultBranch,
			&r.LocalPath, &r.Status, &r.ReportLanguage, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, nil
}

// SearchAnalysisResults searches analysis results by strategy or summary, scoped to a user's repos.
// If repoID is non-empty, results are further filtered to that specific repo.
func (s *PostgresStore) SearchAnalysisResults(ctx context.Context, userID, query, repoID string) ([]AnalysisResultRow, error) {
	pattern := "%" + query + "%"
	args := []interface{}{userID, pattern}
	sqlQuery := `SELECT ar.id, ar.repo_id, ar.strategy, ar.summary,
	             COALESCE(ar.summary_translated, ''), COALESCE(ar.details::text, '{}'), ar.score, ar.created_at
	             FROM analysis_results ar
	             JOIN repos r ON r.id = ar.repo_id
	             WHERE r.user_id = $1
	               AND (ar.strategy ILIKE $2 OR ar.summary ILIKE $2 OR ar.summary_translated ILIKE $2)`

	if repoID != "" {
		sqlQuery += ` AND ar.repo_id = $3`
		args = append(args, repoID)
	}

	sqlQuery += ` ORDER BY ar.created_at DESC LIMIT 200`

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search analysis results: %w", err)
	}
	defer rows.Close()

	var results []AnalysisResultRow
	for rows.Next() {
		var r AnalysisResultRow
		if err := rows.Scan(&r.ID, &r.RepoID, &r.Strategy, &r.Summary, &r.SummaryTranslated, &r.Details, &r.Score, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan analysis result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

// DeleteAnalysisResultsByRepo deletes all analysis results for a repo.
func (s *PostgresStore) DeleteAnalysisResultsByRepo(ctx context.Context, repoID string) error {
	query := `DELETE FROM analysis_results WHERE repo_id = $1`
	_, err := s.db.ExecContext(ctx, query, repoID)
	return err
}
