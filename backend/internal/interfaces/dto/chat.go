package dto

// --- Chat / Agent API ---

// CreateConversationRequest for POST /api/chat/conversations
type CreateConversationRequest struct {
	ImagePath string `json:"imagePath,omitempty"`
}

// ConversationDTO represents a conversation in API responses.
type ConversationDTO struct {
	ID        uint   `json:"id"`
	ImagePath string `json:"imagePath,omitempty"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// SendMessageRequest for POST /api/chat/conversations/:id/messages
type SendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// ChatMessageDTO represents a message in API responses.
type ChatMessageDTO struct {
	ID        uint           `json:"id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []ToolCallInfo `json:"toolCalls,omitempty"`
	CreatedAt string         `json:"createdAt"`
}

// ToolCallInfo describes a tool invocation for the frontend.
type ToolCallInfo struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

// SSEEvent represents a server-sent event from the agent.
type SSEEvent struct {
	Type    string `json:"type"`            // "tool_call", "tool_result", "message", "error", "done"
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Result  string `json:"result,omitempty"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}
