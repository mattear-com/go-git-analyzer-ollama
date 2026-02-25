package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaEndpointConfig holds the configuration for a single Ollama endpoint.
type OllamaEndpointConfig struct {
	BaseURL string // e.g. http://localhost:11434 or https://api.ollama.com
	Model   string // e.g. bge-m3, qwen3
	Token   string // Bearer token for Ollama Cloud (empty = no auth)
}

// OllamaProvider implements port.AIProvider using the Ollama REST API.
// Supports separate endpoints for embed vs chat (different URLs, models, and tokens).
type OllamaProvider struct {
	embed      OllamaEndpointConfig
	chat       OllamaEndpointConfig
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama-backed AI provider with separate embed/chat configs.
func NewOllamaProvider(embed, chat OllamaEndpointConfig) *OllamaProvider {
	return &OllamaProvider{
		embed:      embed,
		chat:       chat,
		httpClient: &http.Client{},
	}
}

// ModelName returns the chat model identifier.
func (o *OllamaProvider) ModelName() string {
	return o.chat.Model
}

// Embed generates a vector embedding for the given text.
func (o *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]interface{}{
		"model": o.embed.Model,
		"input": text,
	}

	body, err := o.post(ctx, o.embed, "/api/embed", payload)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}

	var resp struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama embed: empty response")
	}

	return resp.Embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts in one call.
func (o *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]interface{}{
		"model": o.embed.Model,
		"input": texts,
	}

	body, err := o.post(ctx, o.embed, "/api/embed", payload)
	if err != nil {
		return nil, fmt.Errorf("ollama embed batch: %w", err)
	}

	var resp struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ollama embed batch decode: %w", err)
	}

	return resp.Embeddings, nil
}

// Chat sends a prompt with context chunks and returns the complete response.
func (o *OllamaProvider) Chat(ctx context.Context, systemPrompt string, userPrompt string, contextChunks []string) (string, error) {
	fullPrompt := userPrompt
	if len(contextChunks) > 0 {
		contextStr := ""
		for i, chunk := range contextChunks {
			contextStr += fmt.Sprintf("\n--- Context chunk %d ---\n%s\n", i+1, chunk)
		}
		fullPrompt = fmt.Sprintf("Relevant code context:\n%s\n\nQuestion: %s", contextStr, userPrompt)
	}

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": fullPrompt},
	}

	payload := map[string]interface{}{
		"model":    o.chat.Model,
		"messages": messages,
		"stream":   false,
	}

	body, err := o.post(ctx, o.chat, "/api/chat", payload)
	if err != nil {
		return "", fmt.Errorf("ollama chat: %w", err)
	}

	var resp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("ollama chat decode: %w", err)
	}

	return resp.Message.Content, nil
}

// ChatStream sends a prompt and streams the response token-by-token.
func (o *OllamaProvider) ChatStream(ctx context.Context, systemPrompt string, userPrompt string, contextChunks []string) (<-chan string, error) {
	fullPrompt := userPrompt
	if len(contextChunks) > 0 {
		contextStr := ""
		for i, chunk := range contextChunks {
			contextStr += fmt.Sprintf("\n--- Context chunk %d ---\n%s\n", i+1, chunk)
		}
		fullPrompt = fmt.Sprintf("Relevant code context:\n%s\n\nQuestion: %s", contextStr, userPrompt)
	}

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": fullPrompt},
	}

	payload := map[string]interface{}{
		"model":    o.chat.Model,
		"messages": messages,
		"stream":   true,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.chat.BaseURL+"/api/chat", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("ollama stream: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.chat.Token != "" {
		req.Header.Set("Authorization", "Bearer "+o.chat.Token)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: %w", err)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for decoder.More() {
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := decoder.Decode(&chunk); err != nil {
				return
			}
			if chunk.Message.Content != "" {
				ch <- chunk.Message.Content
			}
			if chunk.Done {
				return
			}
		}
	}()

	return ch, nil
}

// post is a helper for POST requests to an Ollama endpoint (with optional bearer token).
func (o *OllamaProvider) post(ctx context.Context, cfg OllamaEndpointConfig, path string, payload interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+path, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}
