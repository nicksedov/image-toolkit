package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"image-toolkit/internal/infrastructure/llm"
	"image-toolkit/internal/testutil"
)

// mockToolProvider implements ToolProvider for testing.
type mockToolProvider struct {
	tools      []llm.ToolDefinition
	executeMap map[string]func(args json.RawMessage) (string, error)
}

func (m *mockToolProvider) ToolDefinitions() []llm.ToolDefinition {
	return m.tools
}

func (m *mockToolProvider) ExecuteTool(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	if fn, ok := m.executeMap[name]; ok {
		return fn(arguments)
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

func TestAgent_NoToolCalls(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	convSvc := NewConversationService(db)
	conv, _ := convSvc.CreateConversation(1, "/test.jpg")

	mockLLM := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				Message:    llm.ChatMessage{Role: "assistant", Content: "This is a beautiful sunset."},
				StopReason: "end_turn",
			},
		},
	}

	toolProvider := &mockToolProvider{
		tools: []llm.ToolDefinition{
			{Name: "describe_image", Description: "Describe an image"},
		},
		executeMap: map[string]func(json.RawMessage) (string, error){},
	}

	agent := NewAgent(convSvc, toolProvider, DefaultAgentConfig())

	var events []ToolEvent
	resp, err := agent.ProcessMessage(context.Background(), conv.ID, "What is this?", mockLLM, func(e ToolEvent) {
		events = append(events, e)
	})

	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}
	if resp.Message != "This is a beautiful sunset." {
		t.Errorf("unexpected message: %q", resp.Message)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}

	// Verify events: message + done
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "message" {
		t.Errorf("expected 'message' event, got %q", events[0].Type)
	}
	if events[1].Type != "done" {
		t.Errorf("expected 'done' event, got %q", events[1].Type)
	}
}

func TestAgent_SingleToolCall(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	convSvc := NewConversationService(db)
	conv, _ := convSvc.CreateConversation(1, "/test.jpg")

	mockLLM := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				// First response: call a tool
				Message: llm.ChatMessage{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "describe_image", Arguments: json.RawMessage(`{"image_path":"/test.jpg"}`)},
					},
				},
				StopReason: "tool_use",
			},
			{
				// Second response: final text
				Message:    llm.ChatMessage{Role: "assistant", Content: "The image shows a cat."},
				StopReason: "end_turn",
			},
		},
	}

	toolProvider := &mockToolProvider{
		tools: []llm.ToolDefinition{
			{Name: "describe_image", Description: "Describe an image"},
		},
		executeMap: map[string]func(json.RawMessage) (string, error){
			"describe_image": func(args json.RawMessage) (string, error) {
				return "A fluffy cat sitting on a couch", nil
			},
		},
	}

	agent := NewAgent(convSvc, toolProvider, DefaultAgentConfig())

	var events []ToolEvent
	resp, err := agent.ProcessMessage(context.Background(), conv.ID, "What's in this image?", mockLLM, func(e ToolEvent) {
		events = append(events, e)
	})

	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}
	if resp.Message != "The image shows a cat." {
		t.Errorf("unexpected message: %q", resp.Message)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "describe_image" {
		t.Errorf("expected tool name 'describe_image', got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Result != "A fluffy cat sitting on a couch" {
		t.Errorf("unexpected tool result: %q", resp.ToolCalls[0].Result)
	}

	// Verify events: tool_call + tool_result + message + done
	hasToolCall := false
	hasToolResult := false
	for _, e := range events {
		switch e.Type {
		case "tool_call":
			hasToolCall = true
			if e.Name != "describe_image" {
				t.Errorf("tool_call event name: %q", e.Name)
			}
		case "tool_result":
			hasToolResult = true
		}
	}
	if !hasToolCall {
		t.Error("missing tool_call event")
	}
	if !hasToolResult {
		t.Error("missing tool_result event")
	}
}

func TestAgent_MultipleToolCalls(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	convSvc := NewConversationService(db)
	conv, _ := convSvc.CreateConversation(1, "/test.jpg")

	mockLLM := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				// First: generate tags then search
				Message: llm.ChatMessage{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "generate_tags", Arguments: json.RawMessage(`{"image_path":"/test.jpg"}`)},
					},
				},
				StopReason: "tool_use",
			},
			{
				// Second: search by those tags
				Message: llm.ChatMessage{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{ID: "call_2", Name: "search_by_tags", Arguments: json.RawMessage(`{"tags":["cat","couch"]}`)},
					},
				},
				StopReason: "tool_use",
			},
			{
				// Third: final response
				Message:    llm.ChatMessage{Role: "assistant", Content: "Found 3 similar images."},
				StopReason: "end_turn",
			},
		},
	}

	toolProvider := &mockToolProvider{
		tools: []llm.ToolDefinition{
			{Name: "generate_tags", Description: "Generate tags"},
			{Name: "search_by_tags", Description: "Search by tags"},
		},
		executeMap: map[string]func(json.RawMessage) (string, error){
			"generate_tags": func(args json.RawMessage) (string, error) {
				return `["cat","couch","indoor"]`, nil
			},
			"search_by_tags": func(args json.RawMessage) (string, error) {
				return `[{"path":"/img1.jpg"},{"path":"/img2.jpg"},{"path":"/img3.jpg"}]`, nil
			},
		},
	}

	agent := NewAgent(convSvc, toolProvider, DefaultAgentConfig())

	resp, err := agent.ProcessMessage(context.Background(), conv.ID, "Find similar images", mockLLM, nil)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}
	if resp.Message != "Found 3 similar images." {
		t.Errorf("unexpected message: %q", resp.Message)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestAgent_MaxToolRounds(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	convSvc := NewConversationService(db)
	conv, _ := convSvc.CreateConversation(1, "")

	// LLM always returns tool_use, never end_turn
	toolResp := &llm.ChatResponse{
		Message: llm.ChatMessage{
			Role:      "assistant",
			ToolCalls: []llm.ToolCall{{ID: "c1", Name: "test_tool", Arguments: json.RawMessage(`{}`)}},
		},
		StopReason: "tool_use",
	}
	mockLLM := &mockChatClient{
		responses: []*llm.ChatResponse{
			toolResp, toolResp, toolResp, toolResp, toolResp, // enough to exceed max rounds
		},
	}

	toolProvider := &mockToolProvider{
		tools:      []llm.ToolDefinition{{Name: "test_tool"}},
		executeMap: map[string]func(json.RawMessage) (string, error){
			"test_tool": func(args json.RawMessage) (string, error) { return "ok", nil },
		},
	}

	config := AgentConfig{MaxTokens: 8000, MaxToolRounds: 3}
	agent := NewAgent(convSvc, toolProvider, config)

	resp, err := agent.ProcessMessage(context.Background(), conv.ID, "loop forever", mockLLM, nil)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should have hit max rounds and returned fallback
	if resp.Message == "" {
		t.Error("expected fallback message, got empty")
	}
	// Should have exactly 3 tool calls (one per round)
	if len(resp.ToolCalls) != 3 {
		t.Errorf("expected 3 tool calls (max rounds), got %d", len(resp.ToolCalls))
	}
}

func TestAgent_ToolError(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	convSvc := NewConversationService(db)
	conv, _ := convSvc.CreateConversation(1, "/test.jpg")

	mockLLM := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				Message: llm.ChatMessage{
					Role:      "assistant",
					ToolCalls: []llm.ToolCall{{ID: "c1", Name: "describe_image", Arguments: json.RawMessage(`{"image_path":"/missing.jpg"}`)}},
				},
				StopReason: "tool_use",
			},
			{
				Message:    llm.ChatMessage{Role: "assistant", Content: "Sorry, the image was not found."},
				StopReason: "end_turn",
			},
		},
	}

	toolProvider := &mockToolProvider{
		tools: []llm.ToolDefinition{{Name: "describe_image"}},
		executeMap: map[string]func(json.RawMessage) (string, error){
			"describe_image": func(args json.RawMessage) (string, error) {
				return "", fmt.Errorf("image not found")
			},
		},
	}

	agent := NewAgent(convSvc, toolProvider, DefaultAgentConfig())

	resp, err := agent.ProcessMessage(context.Background(), conv.ID, "Describe this", mockLLM, nil)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Tool error should be captured in result
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Result != "Error: image not found" {
		t.Errorf("expected error in tool result, got %q", resp.ToolCalls[0].Result)
	}
}

func TestAgent_NilEventHandler(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	convSvc := NewConversationService(db)
	conv, _ := convSvc.CreateConversation(1, "")

	mockLLM := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				Message:    llm.ChatMessage{Role: "assistant", Content: "Hello!"},
				StopReason: "end_turn",
			},
		},
	}

	toolProvider := &mockToolProvider{
		tools:      []llm.ToolDefinition{},
		executeMap: map[string]func(json.RawMessage) (string, error){},
	}

	agent := NewAgent(convSvc, toolProvider, DefaultAgentConfig())

	// Pass nil event handler - should not panic
	resp, err := agent.ProcessMessage(context.Background(), conv.ID, "Hi", mockLLM, nil)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}
	if resp.Message != "Hello!" {
		t.Errorf("unexpected message: %q", resp.Message)
	}
}

func TestDefaultAgentConfig(t *testing.T) {
	config := DefaultAgentConfig()
	if config.MaxTokens != 8000 {
		t.Errorf("expected MaxTokens=8000, got %d", config.MaxTokens)
	}
	if config.MaxToolRounds != 10 {
		t.Errorf("expected MaxToolRounds=10, got %d", config.MaxToolRounds)
	}
}
