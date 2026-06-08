package llm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient implements Client for OpenAI-compatible API
type OpenAIClient struct {
	BaseURL           string
	APIKey            string
	Model             string
	Timeout           time.Duration
	MaxImageMegapixels float64
}

// NewOpenAIClient creates a new OpenAI client.
// The baseURL is normalized by stripping any trailing /v1 or /v1/ suffix
// so that endpoint paths like /v1/models are not duplicated.
func NewOpenAIClient(baseURL, apiKey, model string, maxImageMegapixels float64) *OpenAIClient {
	baseURL = normalizeOpenAIBaseURL(baseURL)
	return &OpenAIClient{
		BaseURL:            baseURL,
		APIKey:             apiKey,
		Model:              model,
		Timeout:            5 * time.Minute,
		MaxImageMegapixels: maxImageMegapixels,
	}
}

// normalizeOpenAIBaseURL strips trailing slashes and an optional /v1 suffix
// from the base URL. OpenAI-compatible providers may supply a base URL that
// already includes /v1 (e.g. https://host/compatible-mode/v1). Without
// normalization the client would build paths like /v1/v1/models → 404.
func normalizeOpenAIBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	return baseURL
}

// openAIRequest represents OpenAI chat completion request
type openAIRequest struct {
	Model     string               `json:"model"`
	Messages  []openAIChatMessage  `json:"messages"`
	MaxTokens int                  `json:"max_tokens,omitempty"`
	Tools     []openAIToolParam    `json:"tools,omitempty"`
}

type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    any                  `json:"content,omitempty"` // string or []openAIContent for multimodal
	ToolCalls  []openAIToolCallResp `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openAIToolParam struct {
	Type     string              `json:"type"` // "function"
	Function openAIFunctionParam `json:"function"`
}

type openAIFunctionParam struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCallResp struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIMessage struct {
	Role    string          `json:"role"`
	Content []openAIContent `json:"content"`
}

type openAIContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openAIImageURL `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL string `json:"url"`
}

// openAIResponse represents OpenAI chat completion response
type openAIResponse struct {
	Choices []struct {
		Message      openAIChoiceMessage `json:"message"`
		FinishReason string              `json:"finish_reason"`
	} `json:"choices"`
}

type openAIChoiceMessage struct {
	Content   string               `json:"content"`
	ToolCalls []openAIToolCallResp `json:"tool_calls,omitempty"`
}

// Recognize performs OCR using OpenAI-compatible API
func (c *OpenAIClient) Recognize(imagePath string, systemPrompt string, userMessage string) (string, error) {
	// Read and optionally resize image
	imgData, mediaType, err := resizeImageForLLM(imagePath, c.MaxImageMegapixels)
	if err != nil {
		return "", fmt.Errorf("failed to prepare image: %w", err)
	}

	// Encode image to base64 data URL
	base64Img := base64.StdEncoding.EncodeToString(imgData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, base64Img)

	// Prepare request using the multimodal message format
	req := openAIRequest{
		Model: c.Model,
		Messages: []openAIChatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role: "user",
				Content: []openAIContent{
					{Type: "text", Text: userMessage},
					{Type: "image_url", ImageURL: &openAIImageURL{URL: dataURL}},
				},
			},
		},
		MaxTokens: 4000,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	client := &http.Client{Timeout: c.Timeout}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var openAIResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// Chat performs a conversational LLM call with optional tool definitions.
// It implements the ChatClient interface.
func (c *OpenAIClient) Chat(req ChatRequest) (*ChatResponse, error) {
	messages := make([]openAIChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := openAIChatMessage{
			Role:       m.Role,
			ToolCallID: m.ToolCallID,
		}
		if m.Role == "tool" {
			// Tool result messages have string content and a tool_call_id
			msg.Content = m.Content
		} else if len(m.ToolCalls) > 0 {
			// Assistant message requesting tool invocations
			msg.Content = m.Content
			msg.ToolCalls = make([]openAIToolCallResp, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msg.ToolCalls[j] = openAIToolCallResp{
					ID:   tc.ID,
					Type: "function",
				}
				msg.ToolCalls[j].Function.Name = tc.Name
				msg.ToolCalls[j].Function.Arguments = string(tc.Arguments)
			}
		} else {
			msg.Content = m.Content
		}
		messages[i] = msg
	}

	var tools []openAIToolParam
	for _, t := range req.Tools {
		tools = append(tools, openAIToolParam{
			Type: "function",
			Function: openAIFunctionParam{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	oaiReq := openAIRequest{
		Model:     c.Model,
		Messages:  messages,
		MaxTokens: 4000,
		Tools:     tools,
	}

	reqBody, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode chat response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in chat response")
	}

	choice := oaiResp.Choices[0]
	chatResp := &ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: choice.Message.Content,
		},
	}

	if len(choice.Message.ToolCalls) > 0 {
		chatResp.StopReason = "tool_use"
		for _, tc := range choice.Message.ToolCalls {
			chatResp.Message.ToolCalls = append(chatResp.Message.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
	} else {
		chatResp.StopReason = "end_turn"
	}

	return chatResp, nil
}

// openAIModelsResponse represents OpenAI models list response
type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
}

// ListModels returns a list of available models from OpenAI-compatible server
func (c *OpenAIClient) ListModels() ([]ModelInfo, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("GET", c.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		}
	}

	return models, nil
}
