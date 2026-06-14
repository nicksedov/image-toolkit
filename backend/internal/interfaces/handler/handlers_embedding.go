package handler

import (
	"net/http"

	"image-toolkit/internal/application/imaging"

	"github.com/gin-gonic/gin"
)

// handleEmbeddingStatus returns the current embedding backfill status.
func (s *Server) handleEmbeddingStatus(c *gin.Context) {
	if s.embeddingBackfill == nil {
		c.JSON(http.StatusOK, imaging.EmbeddingBackfillStatus{Running: false})
		return
	}
	status := s.embeddingBackfill.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleEmbeddingStart starts the embedding backfill process.
func (s *Server) handleEmbeddingStart(c *gin.Context) {
	if s.embeddingBackfill == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Embedding backfill manager not available"})
		return
	}

	if err := s.embeddingBackfill.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Embedding backfill started"})
}

// handleEmbeddingStop stops the embedding backfill process.
func (s *Server) handleEmbeddingStop(c *gin.Context) {
	if s.embeddingBackfill == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Embedding backfill manager not available"})
		return
	}
	s.embeddingBackfill.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "Embedding backfill stopped"})
}
