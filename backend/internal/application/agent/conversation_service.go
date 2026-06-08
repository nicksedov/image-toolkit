package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// ConversationService manages conversation persistence and context compression.
type ConversationService struct {
	db *gorm.DB
}

// NewConversationService creates a new conversation service.
func NewConversationService(db *gorm.DB) *ConversationService {
	return &ConversationService{db: db}
}

// CreateConversation creates a new conversation for a user.
func (s *ConversationService) CreateConversation(userID uint, imagePath string) (*domain.Conversation, error) {
	conv := &domain.Conversation{
		UserID:    userID,
		ImagePath: imagePath,
		Title:     "New Chat",
	}
	if err := s.db.Create(conv).Error; err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	return conv, nil
}

// GetConversation retrieves a conversation by ID, verifying user ownership.
func (s *ConversationService) GetConversation(convID, userID uint) (*domain.Conversation, error) {
	var conv domain.Conversation
	if err := s.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	return &conv, nil
}

// GetConversationByID retrieves a conversation by ID without user ownership check.
// Use this for internal/agent use where userID is already validated.
func (s *ConversationService) GetConversationByID(convID uint) (*domain.Conversation, error) {
	var conv domain.Conversation
	if err := s.db.First(&conv, convID).Error; err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	return &conv, nil
}

// ListConversations returns all conversations for a user, ordered by most recent.
func (s *ConversationService) ListConversations(userID uint) ([]domain.Conversation, error) {
	var conversations []domain.Conversation
	if err := s.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	return conversations, nil
}

// DeleteConversation deletes a conversation and all its messages.
func (s *ConversationService) DeleteConversation(convID, userID uint) error {
	// Verify ownership
	var conv domain.Conversation
	if err := s.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", convID).Delete(&domain.ConversationMessage{}).Error; err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}
		if err := tx.Delete(&conv).Error; err != nil {
			return fmt.Errorf("failed to delete conversation: %w", err)
		}
		return nil
	})
}

// AddMessage adds a message to a conversation and updates the conversation timestamp.
func (s *ConversationService) AddMessage(convID uint, msg domain.ConversationMessage) error {
	msg.ConversationID = convID
	if err := s.db.Create(&msg).Error; err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	// Update conversation timestamp and title (if first user message)
	s.db.Model(&domain.Conversation{}).
		Where("id = ?", convID).
		Update("updated_at", msg.CreatedAt)

	// Auto-generate title from first user message
	var count int64
	s.db.Model(&domain.ConversationMessage{}).
		Where("conversation_id = ? AND role = ?", convID, "user").
		Count(&count)

	if count == 1 && msg.Role == "user" {
		title := msg.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		s.db.Model(&domain.Conversation{}).
			Where("id = ?", convID).
			Update("title", title)
	}

	return nil
}

// GetMessages returns all messages for a conversation, ordered chronologically.
func (s *ConversationService) GetMessages(convID uint) ([]domain.ConversationMessage, error) {
	var messages []domain.ConversationMessage
	if err := s.db.Where("conversation_id = ?", convID).Order("created_at ASC, id ASC").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	return messages, nil
}

// CountTokens estimates the total token count across all messages in a conversation.
// Uses a rough approximation: ~4 chars per token for English, ~2 chars per token for Russian.
func (s *ConversationService) CountTokens(convID uint) (int, error) {
	var messages []domain.ConversationMessage
	if err := s.db.Where("conversation_id = ?", convID).Find(&messages).Error; err != nil {
		return 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg.Content)
	}
	return total, nil
}

// SummarizeOlderMessages compresses older messages into a summary to reduce context size.
// Keeps the last `keepRecent` messages intact and summarizes everything before them.
func (s *ConversationService) SummarizeOlderMessages(convID uint, keepRecent int, chatClient llm.ChatClient) error {
	var messages []domain.ConversationMessage
	if err := s.db.Where("conversation_id = ?", convID).Order("created_at ASC, id ASC").Find(&messages).Error; err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) <= keepRecent {
		return nil // Nothing to summarize
	}

	toSummarize := messages[:len(messages)-keepRecent]
	toKeep := messages[len(messages)-keepRecent:]

	// Build text to summarize
	var sb strings.Builder
	for _, msg := range toSummarize {
		fmt.Fprintf(&sb, "[%s]: %s\n", msg.Role, msg.Content)
	}

	// Call LLM to summarize
	summary, err := chatClient.Chat(llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{
				Role:    "system",
				Content: "Summarize the following conversation concisely, preserving key information, tool results, and context. Output only the summary text.",
			},
			{
				Role:    "user",
				Content: sb.String(),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to summarize conversation: %w", err)
	}

	// Delete old messages and replace with a single summary message
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete old messages
		var oldIDs []uint
		for _, msg := range toSummarize {
			oldIDs = append(oldIDs, msg.ID)
		}
		if len(oldIDs) > 0 {
			if err := tx.Where("id IN ?", oldIDs).Delete(&domain.ConversationMessage{}).Error; err != nil {
				return fmt.Errorf("failed to delete old messages: %w", err)
			}
		}

		// Insert summary message at the beginning
		summaryMsg := domain.ConversationMessage{
			ConversationID: convID,
			Role:           "system",
			Content:        "[Previous conversation summary]: " + summary.Message.Content,
			TokenCount:     estimateTokens(summary.Message.Content),
		}
		// Set created_at to before the kept messages
		if len(toKeep) > 0 {
			summaryMsg.CreatedAt = toKeep[0].CreatedAt.Add(-1)
		}
		if err := tx.Create(&summaryMsg).Error; err != nil {
			return fmt.Errorf("failed to create summary message: %w", err)
		}

		return nil
	})
}

// MessagesToChatMessages converts domain messages to LLM chat messages.
func MessagesToChatMessages(messages []domain.ConversationMessage) []llm.ChatMessage {
	var result []llm.ChatMessage
	for _, msg := range messages {
		cm := llm.ChatMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// Restore tool calls from JSON if present
		if msg.ToolCallsJSON != "" {
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal([]byte(msg.ToolCallsJSON), &toolCalls); err == nil {
				cm.ToolCalls = toolCalls
			}
		}

		result = append(result, cm)
	}
	return result
}

// estimateTokens provides a rough token count estimate.
func estimateTokens(text string) int {
	// Count Cyrillic vs Latin to adjust estimate
	cyrillicCount := 0
	totalChars := 0
	for _, ch := range text {
		totalChars++
		if ch >= 0x0400 && ch <= 0x04FF {
			cyrillicCount++
		}
	}

	if totalChars == 0 {
		return 0
	}

	// If mostly Cyrillic, use ~2 chars per token; otherwise ~4 chars per token
	if float64(cyrillicCount)/float64(totalChars) > 0.5 {
		return (totalChars + 1) / 2
	}
	return (totalChars + 3) / 4
}
