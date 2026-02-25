package domain

import "time"

// Embedding represents a vectorized chunk of code stored in pgvector.
type Embedding struct {
	ID         string    `json:"id"          db:"id"`
	SnapshotID string    `json:"snapshot_id" db:"snapshot_id"`
	RepoID     string    `json:"repo_id"     db:"repo_id"`
	FilePath   string    `json:"file_path"   db:"file_path"`
	ChunkIndex int       `json:"chunk_index" db:"chunk_index"`
	Content    string    `json:"content"     db:"content"`
	Language   string    `json:"language"    db:"language"`
	Vector     []float32 `json:"-"           db:"vector"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

// SimilarChunk is returned by semantic search, including similarity score.
type SimilarChunk struct {
	Embedding
	Similarity float64 `json:"similarity"`
}
