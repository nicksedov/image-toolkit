package domain

import "time"

// Conversation represents a chat dialog session with the AI agent.
type Conversation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	ImagePath string    `json:"imagePath,omitempty"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ConversationMessage stores individual messages in a conversation.
type ConversationMessage struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID uint      `gorm:"index;not null" json:"conversationId"`
	Role           string    `gorm:"not null" json:"role"` // "user", "assistant", "system", "tool"
	Content        string    `gorm:"type:text" json:"content"`
	ToolCallsJSON  string    `gorm:"type:text" json:"toolCallsJson,omitempty"`
	ToolCallID     string    `json:"toolCallId,omitempty"`
	TokenCount     int       `json:"tokenCount"`
	CreatedAt      time.Time `json:"createdAt"`
}
