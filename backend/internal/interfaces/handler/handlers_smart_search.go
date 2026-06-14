package handler

import (
	"net/http"
	"path/filepath"
	"strconv"

	"image-toolkit/internal/application/imaging"

	"github.com/gin-gonic/gin"
)

// smartSearchImageDTO represents a single result in the smart search response.
type smartSearchImageDTO struct {
	ID         uint     `json:"id"`
	Path       string   `json:"path"`
	FileName   string   `json:"fileName"`
	ModTime    string   `json:"modTime,omitempty"`
	Similarity float64  `json:"similarity"`
	Tags       []string `json:"tags"`
}

// smartSearchResponse is the response for the smart search endpoint.
type smartSearchResponse struct {
	Images []smartSearchImageDTO `json:"images"`
	Total  int                   `json:"total"`
	Query  string                `json:"query"`
}

// handleSmartSearch performs semantic search over image tag embeddings.
func (s *Server) handleSmartSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	result, err := imaging.SearchByEmbedding(s.db, query, limit)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	images := make([]smartSearchImageDTO, 0, len(result.Images))
	for _, img := range result.Images {
		images = append(images, smartSearchImageDTO{
			ID:         img.ImageFileID,
			Path:       img.Path,
			FileName:   filepath.Base(img.Path),
			ModTime:    img.ModTime.Format("2006-01-02 15:04:05"),
			Similarity: img.Similarity,
			Tags:       img.Tags,
		})
	}

	c.JSON(http.StatusOK, smartSearchResponse{
		Images: images,
		Total:  len(images),
		Query:  query,
	})
}
