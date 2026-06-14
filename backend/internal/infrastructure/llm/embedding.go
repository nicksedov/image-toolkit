package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// EmbeddingClient generates vector embeddings for text.
type EmbeddingClient interface {
	Embed(texts []string) ([][]float32, error)
}

// --- Ollama embedding ---

// ollamaEmbedRequest is the Ollama /api/embed request body.
type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// ollamaEmbedResponse is the Ollama /api/embed response body.
type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed generates embeddings using Ollama's /api/embed endpoint.
func (c *OllamaClient) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	reqBody, err := json.Marshal(ollamaEmbedRequest{
		Model: c.Model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/api/embed", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama embed API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embedResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	if len(embedResp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(texts), len(embedResp.Embeddings))
	}

	return embedResp.Embeddings, nil
}

// --- OpenAI embedding ---

// openAIEmbedRequest is the OpenAI /v1/embeddings request body.
type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// openAIEmbedResponse is the OpenAI /v1/embeddings response body.
type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed generates embeddings using OpenAI's /v1/embeddings endpoint.
func (c *OpenAIClient) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	reqBody, err := json.Marshal(openAIEmbedRequest{
		Model: c.Model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v1/embeddings", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI embed API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embedResp openAIEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	if len(embedResp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(texts), len(embedResp.Data))
	}

	embeddings := make([][]float32, len(embedResp.Data))
	for i, d := range embedResp.Data {
		embeddings[i] = d.Embedding
	}

	return embeddings, nil
}

// --- Factory ---

// NewEmbeddingClient creates an embedding client based on provider type.
// Uses the same provider patterns as the VL LLM client.
func NewEmbeddingClient(provider, baseURL, apiKey, model string) (EmbeddingClient, error) {
	switch provider {
	case ProviderOllama:
		return NewOllamaClient(baseURL, "", model, 0), nil
	case ProviderOllamaCloud:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for Ollama Cloud provider")
		}
		return NewOllamaClient(baseURL, apiKey, model, 0), nil
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for OpenAI provider")
		}
		return NewOpenAIClient(baseURL, apiKey, model, 0), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}

// Float32SliceToPgVector converts a float32 slice to pgvector format string.
func Float32SliceToPgVector(vec []float32) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
