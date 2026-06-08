package agent

import (
	"encoding/json"
	"testing"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"
	"image-toolkit/internal/testutil"
)

func TestCreateConversation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)

	conv, err := svc.CreateConversation(1, "/photos/test.jpg")
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	if conv.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if conv.UserID != 1 {
		t.Errorf("expected UserID=1, got %d", conv.UserID)
	}
	if conv.ImagePath != "/photos/test.jpg" {
		t.Errorf("expected ImagePath='/photos/test.jpg', got %q", conv.ImagePath)
	}
	if conv.Title != "New Chat" {
		t.Errorf("expected Title='New Chat', got %q", conv.Title)
	}
}

func TestGetConversation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "/img.jpg")

	// Correct user
	got, err := svc.GetConversation(conv.ID, 1)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}
	if got.ID != conv.ID {
		t.Errorf("expected ID=%d, got %d", conv.ID, got.ID)
	}

	// Wrong user
	_, err = svc.GetConversation(conv.ID, 999)
	if err == nil {
		t.Fatal("expected error for wrong user, got nil")
	}
}

func TestGetConversationByID(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "/img.jpg")

	got, err := svc.GetConversationByID(conv.ID)
	if err != nil {
		t.Fatalf("GetConversationByID failed: %v", err)
	}
	if got.ID != conv.ID {
		t.Errorf("expected ID=%d, got %d", conv.ID, got.ID)
	}

	// Non-existent
	_, err = svc.GetConversationByID(9999)
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
}

func TestListConversations(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	svc.CreateConversation(1, "/img1.jpg")
	svc.CreateConversation(1, "/img2.jpg")
	svc.CreateConversation(2, "/other.jpg")

	list, err := svc.ListConversations(1)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 conversations for user 1, got %d", len(list))
	}

	list2, _ := svc.ListConversations(2)
	if len(list2) != 1 {
		t.Errorf("expected 1 conversation for user 2, got %d", len(list2))
	}
}

func TestDeleteConversation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "/img.jpg")

	// Add some messages
	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "assistant", Content: "hi there"})

	// Wrong user
	err := svc.DeleteConversation(conv.ID, 999)
	if err == nil {
		t.Fatal("expected error deleting with wrong user")
	}

	// Correct user
	err = svc.DeleteConversation(conv.ID, 1)
	if err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}

	// Verify deleted
	_, err = svc.GetConversationByID(conv.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}

	// Verify messages deleted
	msgs, _ := svc.GetMessages(conv.ID)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after deletion, got %d", len(msgs))
	}
}

func TestAddAndGetMessages(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "")

	err := svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "user", Content: "Hello"})
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}
	err = svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "assistant", Content: "Hi!"})
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	msgs, err := svc.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Hi!" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestAutoTitleFromFirstUserMessage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "")

	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "user", Content: "What is in this photo?"})

	got, _ := svc.GetConversationByID(conv.ID)
	if got.Title != "What is in this photo?" {
		t.Errorf("expected auto-title, got %q", got.Title)
	}
}

func TestAutoTitleTruncated(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "")

	longMsg := "This is a very long message that exceeds fifty characters and should be truncated"
	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "user", Content: longMsg})

	got, _ := svc.GetConversationByID(conv.ID)
	if len(got.Title) > 54 { // 50 + "..."
		t.Errorf("title too long: %q (len=%d)", got.Title, len(got.Title))
	}
}

func TestCountTokens(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "")

	// Add known-length messages
	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "user", Content: "hello world"})        // ~3 tokens
	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "assistant", Content: "how can I help"}) // ~4 tokens

	count, err := svc.CountTokens(conv.ID)
	if err != nil {
		t.Fatalf("CountTokens failed: %v", err)
	}
	if count < 2 || count > 20 {
		t.Errorf("unexpected token count: %d", count)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		minExp int
		maxExp int
	}{
		{"empty", "", 0, 0},
		{"short english", "hello", 1, 3},
		{"sentence", "The quick brown fox jumps over the lazy dog", 8, 14},
		{"russian", "Привет мир", 4, 7}, // ~10 chars, ~2 chars/token = 5
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.text)
			if got < tt.minExp || got > tt.maxExp {
				t.Errorf("estimateTokens(%q) = %d, want [%d, %d]", tt.text, got, tt.minExp, tt.maxExp)
			}
		})
	}
}

func TestMessagesToChatMessages(t *testing.T) {
	messages := []domain.ConversationMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{
			Role:          "assistant",
			Content:       "",
			ToolCallsJSON: `[{"id":"call_1","name":"describe_image","arguments":{"image_path":"/test.jpg"}}]`,
		},
		{Role: "tool", Content: "A beautiful sunset", ToolCallID: "call_1"},
	}

	result := MessagesToChatMessages(messages)

	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// Check tool calls restored
	if len(result[2].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[2].ToolCalls))
	}
	if result[2].ToolCalls[0].Name != "describe_image" {
		t.Errorf("expected tool name 'describe_image', got %q", result[2].ToolCalls[0].Name)
	}

	// Check tool result
	if result[3].ToolCallID != "call_1" {
		t.Errorf("expected ToolCallID='call_1', got %q", result[3].ToolCallID)
	}
}

func TestMessagesToChatMessages_InvalidJSON(t *testing.T) {
	messages := []domain.ConversationMessage{
		{
			Role:          "assistant",
			ToolCallsJSON: "invalid json",
		},
	}

	result := MessagesToChatMessages(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	// Invalid JSON should be silently ignored
	if len(result[0].ToolCalls) != 0 {
		t.Errorf("expected no tool calls for invalid JSON, got %d", len(result[0].ToolCalls))
	}
}

// mockChatClient implements llm.ChatClient for testing.
type mockChatClient struct {
	responses []*llm.ChatResponse
	callIndex int
}

func (m *mockChatClient) Chat(req llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.callIndex >= len(m.responses) {
		// Default: end turn with empty message
		return &llm.ChatResponse{
			Message:    llm.ChatMessage{Role: "assistant", Content: "done"},
			StopReason: "end_turn",
		}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func (m *mockChatClient) Recognize(imagePath, systemPrompt, userMessage string) (string, error) {
	return "", nil
}

func (m *mockChatClient) ListModels() ([]llm.ModelInfo, error) {
	return nil, nil
}

func TestSummarizeOlderMessages(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "")

	// Add 8 messages
	for i := 0; i < 8; i++ {
		svc.AddMessage(conv.ID, domain.ConversationMessage{
			Role:    "user",
			Content: "message " + string(rune('A'+i)),
		})
	}

	mock := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				Message:    llm.ChatMessage{Role: "assistant", Content: "Summary of earlier messages"},
				StopReason: "end_turn",
			},
		},
	}

	// Keep last 4, summarize the first 4
	err := svc.SummarizeOlderMessages(conv.ID, 4, mock)
	if err != nil {
		t.Fatalf("SummarizeOlderMessages failed: %v", err)
	}

	msgs, _ := svc.GetMessages(conv.ID)
	// Should have: 1 summary + 4 kept = 5
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages after summarization, got %d", len(msgs))
	}

	// First message should be the summary
	if msgs[0].Role != "system" {
		t.Errorf("expected first message to be system summary, got role=%q", msgs[0].Role)
	}
}

func TestSummarizeOlderMessages_NothingToSummarize(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(1, "")

	svc.AddMessage(conv.ID, domain.ConversationMessage{Role: "user", Content: "only one"})

	mock := &mockChatClient{}
	err := svc.SummarizeOlderMessages(conv.ID, 6, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have 1 message
	msgs, _ := svc.GetMessages(conv.ID)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

// Ensure json.RawMessage tool call arguments survive the round-trip
func TestToolCallsJSON_RoundTrip(t *testing.T) {
	args := json.RawMessage(`{"image_path":"/test.jpg","language":"en"}`)
	toolCalls := []llm.ToolCall{
		{ID: "call_123", Name: "describe_image", Arguments: args},
	}

	data, _ := json.Marshal(toolCalls)
	var restored []llm.ToolCall
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(restored) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(restored))
	}
	if restored[0].Name != "describe_image" {
		t.Errorf("expected name 'describe_image', got %q", restored[0].Name)
	}
	if string(restored[0].Arguments) != string(args) {
		t.Errorf("arguments mismatch: %s vs %s", restored[0].Arguments, args)
	}
}
