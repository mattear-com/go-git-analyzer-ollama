package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/adapter/store"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

// RAGService handles retrieval-augmented generation over vectorized code.
type RAGService struct {
	ai          port.AIProvider
	vectorStore *store.VectorStore
}

// NewRAGService creates a new RAG service.
func NewRAGService(ai port.AIProvider, vectorStore *store.VectorStore) *RAGService {
	return &RAGService{ai: ai, vectorStore: vectorStore}
}

// Query performs a semantic search + AI chat over a repository's code.
func (s *RAGService) Query(ctx context.Context, repoID, question string) (string, []domain.SimilarChunk, error) {
	slog.Info("RAG query", "repo_id", repoID, "question", question)

	// 1. Embed the question
	queryVector, err := s.ai.Embed(ctx, question)
	if err != nil {
		return "", nil, fmt.Errorf("embed query: %w", err)
	}

	// 2. Retrieve similar code chunks
	chunks, err := s.vectorStore.SearchSimilar(ctx, repoID, queryVector, 10)
	if err != nil {
		return "", nil, fmt.Errorf("search similar: %w", err)
	}

	if len(chunks) == 0 {
		return "No relevant code found for this query.", nil, nil
	}

	// 3. Build context from retrieved chunks
	contextParts := make([]string, len(chunks))
	for i, chunk := range chunks {
		contextParts[i] = fmt.Sprintf("// File: %s (similarity: %.2f)\n%s", chunk.FilePath, chunk.Similarity, chunk.Content)
	}

	// 4. Generate AI response with context
	systemPrompt := `You are CodeLens AI, an expert code analyst. Answer questions about the codebase using the provided code context. 
Be precise, reference specific files and functions, and provide code examples when relevant.
Always cite the source file when referencing code.`

	response, err := s.ai.Chat(ctx, systemPrompt, question, contextParts)
	if err != nil {
		return "", nil, fmt.Errorf("chat: %w", err)
	}

	return response, chunks, nil
}

// QueryStream performs RAG with streaming response.
func (s *RAGService) QueryStream(ctx context.Context, repoID, question string) (<-chan string, []domain.SimilarChunk, error) {
	// 1. Embed the question
	queryVector, err := s.ai.Embed(ctx, question)
	if err != nil {
		return nil, nil, fmt.Errorf("embed query: %w", err)
	}

	// 2. Retrieve similar code chunks
	chunks, err := s.vectorStore.SearchSimilar(ctx, repoID, queryVector, 10)
	if err != nil {
		return nil, nil, fmt.Errorf("search similar: %w", err)
	}

	// 3. Build context
	contextParts := make([]string, len(chunks))
	for i, chunk := range chunks {
		contextParts[i] = fmt.Sprintf("// File: %s\n%s", chunk.FilePath, chunk.Content)
	}

	systemPrompt := `You are CodeLens AI, an expert code analyst. Answer questions about the codebase using the provided code context.
Be precise, reference specific files and functions.`

	// 4. Stream AI response
	stream, err := s.ai.ChatStream(ctx, systemPrompt, question, contextParts)
	if err != nil {
		return nil, nil, fmt.Errorf("chat stream: %w", err)
	}

	return stream, chunks, nil
}

// IndexChunks vectorizes and stores code chunks for a snapshot.
func (s *RAGService) IndexChunks(ctx context.Context, repoID, snapshotID string, files map[string]string) error {
	slog.Info("indexing chunks", "repo_id", repoID, "files", len(files))

	for filePath, content := range files {
		chunks := chunkCode(content, 512)
		if len(chunks) == 0 {
			continue
		}

		vectors, err := s.ai.EmbedBatch(ctx, chunks)
		if err != nil {
			slog.Error("embed batch failed", "file", filePath, "error", err)
			continue
		}

		embeddings := make([]domain.Embedding, len(chunks))
		for i, chunk := range chunks {
			embeddings[i] = domain.Embedding{
				SnapshotID: snapshotID,
				RepoID:     repoID,
				FilePath:   filePath,
				ChunkIndex: i,
				Content:    chunk,
				Language:   detectLanguage(filePath),
				Vector:     vectors[i],
			}
		}

		if err := s.vectorStore.StoreBatchEmbeddings(ctx, embeddings); err != nil {
			slog.Error("store embeddings failed", "file", filePath, "error", err)
			continue
		}
	}

	return nil
}

// chunkCode splits code into overlapping chunks of approximately maxTokens words.
func chunkCode(content string, maxTokens int) []string {
	lines := strings.Split(content, "\n")
	var chunks []string
	var current []string
	currentLen := 0

	for _, line := range lines {
		wordCount := len(strings.Fields(line))
		if currentLen+wordCount > maxTokens && len(current) > 0 {
			chunks = append(chunks, strings.Join(current, "\n"))
			// Keep last 3 lines for overlap
			overlap := 3
			if len(current) < overlap {
				overlap = len(current)
			}
			current = current[len(current)-overlap:]
			currentLen = 0
			for _, l := range current {
				currentLen += len(strings.Fields(l))
			}
		}
		current = append(current, line)
		currentLen += wordCount
	}

	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n"))
	}
	return chunks
}

// detectLanguage infers the programming language from file extension.
func detectLanguage(filePath string) string {
	ext := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(ext, ".go"):
		return "go"
	case strings.HasSuffix(ext, ".ts"), strings.HasSuffix(ext, ".tsx"):
		return "typescript"
	case strings.HasSuffix(ext, ".js"), strings.HasSuffix(ext, ".jsx"):
		return "javascript"
	case strings.HasSuffix(ext, ".py"):
		return "python"
	case strings.HasSuffix(ext, ".rs"):
		return "rust"
	case strings.HasSuffix(ext, ".java"):
		return "java"
	case strings.HasSuffix(ext, ".rb"):
		return "ruby"
	case strings.HasSuffix(ext, ".sql"):
		return "sql"
	case strings.HasSuffix(ext, ".yaml"), strings.HasSuffix(ext, ".yml"):
		return "yaml"
	case strings.HasSuffix(ext, ".json"):
		return "json"
	case strings.HasSuffix(ext, ".md"):
		return "markdown"
	default:
		return "unknown"
	}
}
