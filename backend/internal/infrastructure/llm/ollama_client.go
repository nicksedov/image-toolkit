package llm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient implements Client for Ollama API
type OllamaClient struct {
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	MaxImageMegapixels float64
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, apiKey, model string, maxImageMegapixels float64) *OllamaClient {
	return &OllamaClient{
		BaseURL:            baseURL,
		APIKey:             apiKey,
		Model:              model,
		Timeout:            5 * time.Minute, // VL models can be slow
		MaxImageMegapixels: maxImageMegapixels,
	}
}

// ollamaRequest represents Ollama chat request
type ollamaRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaMessage     `json:"messages"`
	Stream   bool                `json:"stream"`
	Tools    []ollamaToolParam   `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	Images    []string            `json:"images,omitempty"`    // Base64 encoded images
	ToolCalls []ollamaToolCallMsg `json:"tool_calls,omitempty"` // Populated when assistant requests tool use
}

type ollamaToolParam struct {
	Type     string             `json:"type"` // "function"
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ollamaToolCallMsg struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// ollamaResponse represents Ollama chat response
type ollamaResponse struct {
	Message ollamaResponseMessage `json:"message"`
	Done    bool                  `json:"done"`
}

type ollamaResponseMessage struct {
	Content   string              `json:"content"`
	ToolCalls []ollamaToolCallMsg `json:"tool_calls,omitempty"`
}

// Recognize performs OCR using Ollama
func (c *OllamaClient) Recognize(imagePath string, systemPrompt string, userMessage string) (string, error) {
	// Read and optionally resize image
	imgData, _, err := resizeImageForLLM(imagePath, c.MaxImageMegapixels)
	if err != nil {
		return "", fmt.Errorf("failed to prepare image: %w", err)
	}

	// Encode image to base64
	base64Img := base64.StdEncoding.EncodeToString(imgData)

	// Prepare request
	req := ollamaRequest{
		Model: c.Model,
		Messages: []ollamaMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userMessage,
				Images:  []string{base64Img},
			},
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	client := &http.Client{Timeout: c.Timeout}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/api/chat", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return ollamaResp.Message.Content, nil
}

// Chat performs a conversational LLM call with optional tool definitions.
// It implements the ChatClient interface.
func (c *OllamaClient) Chat(req ChatRequest) (*ChatResponse, error) {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		// If this is an assistant message with tool calls, populate tool_calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal(tc.Arguments, &args); err != nil {
					args = map[string]any{"raw": string(tc.Arguments)}
				}
				tcMsg := ollamaToolCallMsg{}
				tcMsg.Function.Name = tc.Name
				tcMsg.Function.Arguments = args
				msg.ToolCalls = append(msg.ToolCalls, tcMsg)
			}
		}
		messages[i] = msg
	}

	var tools []ollamaToolParam
	for _, t := range req.Tools {
		tools = append(tools, ollamaToolParam{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	oReq := ollamaRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}

	reqBody, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/api/chat", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var oResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("failed to decode chat response: %w", err)
	}

	chatResp := &ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: oResp.Message.Content,
		},
	}

	if len(oResp.Message.ToolCalls) > 0 {
		chatResp.StopReason = "tool_use"
		for i, tc := range oResp.Message.ToolCalls {
			argsJSON, err := json.Marshal(tc.Function.Arguments)
			if err != nil {
				argsJSON = []byte("{}")
			}
			chatResp.Message.ToolCalls = append(chatResp.Message.ToolCalls, ToolCall{
				ID:        fmt.Sprintf("call_%d", i),
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(argsJSON),
			})
		}
	} else {
		chatResp.StopReason = "end_turn"
	}

	return chatResp, nil
}

// ollamaTagsResponse represents Ollama tags response
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	Digest     string `json:"digest"`
}

// ListModels returns a list of available models from Ollama server
func (c *OllamaClient) ListModels() ([]ModelInfo, error) {
	httpReq, err := http.NewRequest("GET", c.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = ModelInfo{
			ID:   m.Name,
			Name: m.Name,
			Size: m.Size,
		}
	}

	return models, nil
}
