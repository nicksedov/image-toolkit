package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"
)

// AgentConfig holds agent configuration.
type AgentConfig struct {
	MaxTokens     int // Token threshold for summarization (default 8000)
	MaxToolRounds int // Maximum tool-use iterations per message (default 10)
}

// DefaultAgentConfig returns sensible defaults.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxTokens:     8000,
		MaxToolRounds: 10,
	}
}

// ToolCallInfo describes a tool invocation for the frontend.
type ToolCallInfo struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

// AgentResponse is the result of processing a user message.
type AgentResponse struct {
	Message   string         `json:"message"`
	ToolCalls []ToolCallInfo `json:"toolCalls,omitempty"`
}

// ToolEvent represents a real-time tool execution event streamed to the frontend.
type ToolEvent struct {
	Type   string `json:"type"` // "tool_call", "tool_result", "message", "error", "done"
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
	Result string `json:"result,omitempty"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ToolEventHandler is called during agent execution to stream events.
type ToolEventHandler func(event ToolEvent)

// ToolProvider supplies tool definitions and execution to the agent.
// Implemented by mcpserver.ImageToolkitMCPServer.
type ToolProvider interface {
	ToolDefinitions() []llm.ToolDefinition
	ExecuteTool(ctx context.Context, name string, arguments json.RawMessage) (string, error)
}

// Agent orchestrates conversational AI with MCP tool invocations.
type Agent struct {
	conversationService *ConversationService
	toolProvider        ToolProvider
	config              AgentConfig
}

// NewAgent creates a new agent instance.
func NewAgent(convService *ConversationService, toolProvider ToolProvider, config AgentConfig) *Agent {
	return &Agent{
		conversationService: convService,
		toolProvider:        toolProvider,
		config:              config,
	}
}

// ProcessMessage handles a user message, runs the agent loop, and returns the assistant response.
// If eventHandler is non-nil, it receives real-time events during processing.
func (a *Agent) ProcessMessage(ctx context.Context, convID uint, userMessage string, chatClient llm.ChatClient, eventHandler ToolEventHandler) (*AgentResponse, error) {
	// Save user message
	userMsg := domain.ConversationMessage{
		Role:       "user",
		Content:    userMessage,
		TokenCount: estimateTokens(userMessage),
	}
	if err := a.conversationService.AddMessage(convID, userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// Get conversation context
	conv, err := a.conversationService.GetConversationByID(convID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	// Load message history
	messages, err := a.conversationService.GetMessages(convID)
	if err != nil {
		return nil, fmt.Errorf("failed to load message history: %w", err)
	}

	// Build system prompt
	systemPrompt := a.buildSystemPrompt(conv)

	// Get tool definitions from tool provider
	toolDefs := a.toolProvider.ToolDefinitions()

	// Convert to chat messages
	chatMessages := MessagesToChatMessages(messages)

	// Prepend system prompt
	fullMessages := append([]llm.ChatMessage{{Role: "system", Content: systemPrompt}}, chatMessages...)

	var allToolCalls []ToolCallInfo

	// Agent loop: iterate until the LLM produces a final text response or max rounds exceeded
	for round := 0; round < a.config.MaxToolRounds; round++ {
		resp, err := chatClient.Chat(llm.ChatRequest{
			Messages: fullMessages,
			Tools:    toolDefs,
		})
		if err != nil {
			if eventHandler != nil {
				eventHandler(ToolEvent{Type: "error", Error: err.Error()})
			}
			return nil, fmt.Errorf("LLM chat failed: %w", err)
		}

		if resp.StopReason == "end_turn" || len(resp.Message.ToolCalls) == 0 {
			// Final text response
			assistantMsg := domain.ConversationMessage{
				Role:       "assistant",
				Content:    resp.Message.Content,
				TokenCount: estimateTokens(resp.Message.Content),
			}
			if err := a.conversationService.AddMessage(convID, assistantMsg); err != nil {
				log.Printf("Failed to save assistant message: %v", err)
			}

			if eventHandler != nil {
				eventHandler(ToolEvent{Type: "message", Content: resp.Message.Content})
				eventHandler(ToolEvent{Type: "done"})
			}

			// Check token threshold and summarize if needed
			a.maybeSummarize(convID, chatClient)

			return &AgentResponse{
				Message:   resp.Message.Content,
				ToolCalls: allToolCalls,
			}, nil
		}

		// Tool use: execute each tool and continue the loop
		// Save assistant message with tool calls
		toolCallsJSON, _ := json.Marshal(resp.Message.ToolCalls)
		assistantMsg := domain.ConversationMessage{
			Role:          "assistant",
			Content:       resp.Message.Content,
			ToolCallsJSON: string(toolCallsJSON),
			TokenCount:    estimateTokens(resp.Message.Content),
		}
		if err := a.conversationService.AddMessage(convID, assistantMsg); err != nil {
			log.Printf("Failed to save assistant tool_call message: %v", err)
		}

		// Add assistant message with tool calls to fullMessages
		fullMessages = append(fullMessages, resp.Message)

		// Execute each tool call
		for _, tc := range resp.Message.ToolCalls {
			if eventHandler != nil {
				eventHandler(ToolEvent{
					Type:   "tool_call",
					Name:   tc.Name,
					Status: "running",
				})
			}

			result, execErr := a.toolProvider.ExecuteTool(ctx, tc.Name, tc.Arguments)

			if execErr != nil {
				result = fmt.Sprintf("Error: %s", execErr.Error())
			}

			if eventHandler != nil {
				eventHandler(ToolEvent{
					Type:   "tool_result",
					Name:   tc.Name,
					Status: "completed",
					Result: result,
				})
			}

			allToolCalls = append(allToolCalls, ToolCallInfo{
				Name:      tc.Name,
				Arguments: string(tc.Arguments),
				Result:    result,
			})

			// Save tool result message
			toolResultMsg := domain.ConversationMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				TokenCount: estimateTokens(result),
			}
			if err := a.conversationService.AddMessage(convID, toolResultMsg); err != nil {
				log.Printf("Failed to save tool result: %v", err)
			}

			// Add tool result to fullMessages for the next LLM call
			fullMessages = append(fullMessages, llm.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max rounds exceeded
	fallbackMsg := "I've reached the maximum number of tool invocations. Here's what I found so far."
	if eventHandler != nil {
		eventHandler(ToolEvent{Type: "message", Content: fallbackMsg})
		eventHandler(ToolEvent{Type: "done"})
	}

	return &AgentResponse{
		Message:   fallbackMsg,
		ToolCalls: allToolCalls,
	}, nil
}

// buildSystemPrompt creates the system prompt for the agent based on conversation context.
func (a *Agent) buildSystemPrompt(conv *domain.Conversation) string {
	prompt := `You are an AI assistant for the Image Toolkit application. You help users analyze, search, and understand their image collection.

You have access to the following tools:
- describe_image: Generate a detailed description of an image
- recognize_text: Extract text from an image (OCR)
- generate_tags: Generate descriptive tags for an image
- ask_about_image: Answer a specific question about an image
- search_by_tags: Find images by their AI-generated tags
- search_by_date: Find images taken within a date range
- search_by_location: Find images at specific geographic coordinates
- search_by_path: Find images by filename or path pattern
- get_image_metadata: Get EXIF metadata for an image

Guidelines:
- Use tools when you need information. Don't guess.
- Be helpful, specific, and accurate.
- Respond in the same language as the user's message.
- When presenting search results, list images with their paths and key metadata.`

	if conv.ImagePath != "" {
		prompt += fmt.Sprintf("\n\nWhen the user refers to \"this image\" or \"the image\", they mean: %s", conv.ImagePath)
	}

	return prompt
}

// maybeSummarize checks token count and triggers summarization if threshold exceeded.
func (a *Agent) maybeSummarize(convID uint, chatClient llm.ChatClient) {
	tokenCount, err := a.conversationService.CountTokens(convID)
	if err != nil {
		log.Printf("Failed to count tokens for conversation %d: %v", convID, err)
		return
	}

	if tokenCount > a.config.MaxTokens {
		log.Printf("Conversation %d has %d tokens (threshold %d), summarizing...", convID, tokenCount, a.config.MaxTokens)
		if err := a.conversationService.SummarizeOlderMessages(convID, 6, chatClient); err != nil {
			log.Printf("Failed to summarize conversation %d: %v", convID, err)
		}
	}
}
