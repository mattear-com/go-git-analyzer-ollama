package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
)

// VectorStore handles pgvector-specific operations for embeddings.
type VectorStore struct {
	store     *PostgresStore
	dimension int
}

// NewVectorStore creates a vector store backed by the given Postgres store.
func NewVectorStore(store *PostgresStore, dimension int) *VectorStore {
	return &VectorStore{store: store, dimension: dimension}
}

// StoreEmbedding persists a single embedding record with its vector.
func (v *VectorStore) StoreEmbedding(ctx context.Context, e *domain.Embedding) error {
	vectorStr := vectorToString(e.Vector)
	query := `INSERT INTO embeddings (snapshot_id, repo_id, file_path, chunk_index, content, language, vector)
	          VALUES ($1, $2, $3, $4, $5, $6, $7::vector)`

	_, err := v.store.db.ExecContext(ctx, query,
		e.SnapshotID, e.RepoID, e.FilePath, e.ChunkIndex, e.Content, e.Language, vectorStr,
	)
	if err != nil {
		return fmt.Errorf("store embedding: %w", err)
	}
	return nil
}

// StoreBatchEmbeddings persists multiple embeddings efficiently.
func (v *VectorStore) StoreBatchEmbeddings(ctx context.Context, embeddings []domain.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tx, err := v.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO embeddings (snapshot_id, repo_id, file_path, chunk_index, content, language, vector)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::vector)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, e := range embeddings {
		vectorStr := vectorToString(e.Vector)
		if _, err := stmt.ExecContext(ctx,
			e.SnapshotID, e.RepoID, e.FilePath, e.ChunkIndex, e.Content, e.Language, vectorStr,
		); err != nil {
			return fmt.Errorf("insert embedding: %w", err)
		}
	}

	return tx.Commit()
}

// SearchSimilar performs a cosine similarity search on embeddings.
func (v *VectorStore) SearchSimilar(ctx context.Context, repoID string, queryVector []float32, limit int) ([]domain.SimilarChunk, error) {
	vectorStr := vectorToString(queryVector)
	query := `SELECT e.id, e.snapshot_id, e.repo_id, e.file_path, e.chunk_index, e.content, e.language, e.created_at,
	                 1 - (e.vector <=> $1::vector) AS similarity
	          FROM embeddings e
	          WHERE e.repo_id = $2
	          ORDER BY e.vector <=> $1::vector
	          LIMIT $3`

	rows, err := v.store.db.QueryContext(ctx, query, vectorStr, repoID, limit)
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}
	defer rows.Close()

	var results []domain.SimilarChunk
	for rows.Next() {
		var sc domain.SimilarChunk
		if err := rows.Scan(
			&sc.ID, &sc.SnapshotID, &sc.RepoID, &sc.FilePath, &sc.ChunkIndex,
			&sc.Content, &sc.Language, &sc.CreatedAt, &sc.Similarity,
		); err != nil {
			return nil, fmt.Errorf("scan similar: %w", err)
		}
		results = append(results, sc)
	}
	return results, nil
}

// DeleteEmbeddingsByRepo deletes all embeddings for a repo.
func (v *VectorStore) DeleteEmbeddingsByRepo(ctx context.Context, repoID string) error {
	query := `DELETE FROM embeddings WHERE repo_id = $1`
	_, err := v.store.db.ExecContext(ctx, query, repoID)
	return err
}

// vectorToString converts a float32 slice to pgvector string format: [0.1,0.2,0.3].
func vectorToString(v []float32) string {
	parts := make([]string, len(v))
	for i, val := range v {
		parts[i] = fmt.Sprintf("%g", val)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
