package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"image-toolkit/internal/application/agent"
	"image-toolkit/internal/infrastructure/llm"
	"image-toolkit/internal/interfaces/dto"
	"image-toolkit/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
)

// handleCreateConversation handles POST /api/chat/conversations
func (s *Server) handleCreateConversation(c *gin.Context) {
	var req dto.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userID := middleware.GetUserID(c)
	conv, err := s.conversationService.CreateConversation(userID, req.ImagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.ConversationDTO{
		ID:        conv.ID,
		ImagePath: conv.ImagePath,
		Title:     conv.Title,
		CreatedAt: conv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: conv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// handleListConversations handles GET /api/chat/conversations
func (s *Server) handleListConversations(c *gin.Context) {
	userID := middleware.GetUserID(c)
	conversations, err := s.conversationService.ListConversations(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]dto.ConversationDTO, len(conversations))
	for i, conv := range conversations {
		result[i] = dto.ConversationDTO{
			ID:        conv.ID,
			ImagePath: conv.ImagePath,
			Title:     conv.Title,
			CreatedAt: conv.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: conv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, result)
}

// handleDeleteConversation handles DELETE /api/chat/conversations/:id
func (s *Server) handleDeleteConversation(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if err := s.conversationService.DeleteConversation(uint(convID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

// handleGetMessages handles GET /api/chat/conversations/:id/messages
func (s *Server) handleGetMessages(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation ID"})
		return
	}

	// Verify ownership
	userID := middleware.GetUserID(c)
	if _, err := s.conversationService.GetConversation(uint(convID), userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	messages, err := s.conversationService.GetMessages(uint(convID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]dto.ChatMessageDTO, len(messages))
	for i, msg := range messages {
		d := dto.ChatMessageDTO{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		// Parse tool calls from JSON
		if msg.ToolCallsJSON != "" {
			var toolCalls []dto.ToolCallInfo
			if err := json.Unmarshal([]byte(msg.ToolCallsJSON), &toolCalls); err == nil {
				d.ToolCalls = toolCalls
			}
		}

		result[i] = d
	}

	c.JSON(http.StatusOK, result)
}

// handleSendMessage handles POST /api/chat/conversations/:id/messages with SSE streaming.
func (s *Server) handleSendMessage(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation ID"})
		return
	}

	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	// Verify ownership
	userID := middleware.GetUserID(c)
	if _, err := s.conversationService.GetConversation(uint(convID), userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	// Create LLM chat client from active provider
	client, _, ok := s.llmFactory.CreateClient(c)
	if !ok {
		return // Error already written by CreateClient
	}

	chatClient, ok := llm.NewChatClient(client)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM provider does not support chat/function calling"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Stream events
	c.Stream(func(w io.Writer) bool {
		eventHandler := func(event agent.ToolEvent) {
			data, _ := json.Marshal(dto.SSEEvent{
				Type:    event.Type,
				Name:    event.Name,
				Status:  event.Status,
				Result:  event.Result,
				Content: event.Content,
				Error:   event.Error,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		_, err := s.agent.ProcessMessage(c.Request.Context(), uint(convID), req.Content, chatClient, eventHandler)
		if err != nil {
			data, _ := json.Marshal(dto.SSEEvent{
				Type:  "error",
				Error: err.Error(),
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		return false // Stop streaming
	})
}
