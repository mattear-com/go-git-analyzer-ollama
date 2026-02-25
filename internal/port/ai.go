package port

import "context"

// AIProvider abstracts the AI/LLM backend for embeddings and chat completions.
// Implementations can target Ollama, OpenAI, or any compatible API.
type AIProvider interface {
	// ModelName returns the identifier of the model being used.
	ModelName() string

	// Embed generates a vector embedding for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts in one call.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Chat sends a prompt with optional context chunks and returns the LLM response.
	Chat(ctx context.Context, systemPrompt string, userPrompt string, contextChunks []string) (string, error)

	// ChatStream sends a prompt and streams the response token-by-token via channel.
	ChatStream(ctx context.Context, systemPrompt string, userPrompt string, contextChunks []string) (<-chan string, error)
}
