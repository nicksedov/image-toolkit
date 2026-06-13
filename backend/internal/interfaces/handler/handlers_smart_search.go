package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"

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

	// Load embedding settings
	var settings domain.LlmSettings
	if err := s.db.First(&settings).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM settings not found"})
		return
	}

	providerAlias := settings.EmbeddingProviderAlias
	if providerAlias == "" {
		providerAlias = settings.ActiveProvider
	}

	var provider domain.LlmProvider
	if err := s.db.Where("alias = ?", providerAlias).First(&provider).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("Embedding provider '%s' not found", providerAlias)})
		return
	}

	modelName := settings.EmbeddingModel
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	embeddingClient, err := llm.NewEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("Failed to create embedding client: %v", err)})
		return
	}

	// Embed the query
	queryEmbeddings, err := embeddingClient.Embed([]string{strings.ToLower(query)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to embed query: %v", err)})
		return
	}
	if len(queryEmbeddings) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Empty embedding result"})
		return
	}

	// Convert to pgvector format for the SQL query
	vecStr := llm.Float32SliceToPgVector(queryEmbeddings[0])

	// Run nearest-neighbor search
	type searchResult struct {
		ImageFileID uint    `gorm:"column:image_file_id"`
		Similarity  float64 `gorm:"column:similarity"`
	}

	var results []searchResult
	if err := s.db.Raw(`
		SELECT te.image_file_id, 1 - (te.embedding <=> ?::vector) AS similarity
		FROM tag_embeddings te
		ORDER BY te.embedding <=> ?::vector
		LIMIT ?
	`, vecStr, vecStr, limit).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Semantic search query failed"})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, smartSearchResponse{
			Images: []smartSearchImageDTO{},
			Total:  0,
			Query:  query,
		})
		return
	}

	// Collect image IDs and fetch image_files data
	imageIDs := make([]uint, len(results))
	similarityMap := make(map[uint]float64)
	for i, r := range results {
		imageIDs[i] = r.ImageFileID
		similarityMap[r.ImageFileID] = r.Similarity
	}

	var files []domain.ImageFile
	s.db.Where("id IN ?", imageIDs).Find(&files)

	fileMap := make(map[uint]domain.ImageFile)
	for _, f := range files {
		fileMap[f.ID] = f
	}

	// Batch-fetch tags for all result images (avoids N+1)
	var allTags []domain.ImageTag
	s.db.Where("image_file_id IN ?", imageIDs).Find(&allTags)
	tagsMap := make(map[uint][]string)
	for _, t := range allTags {
		tagsMap[t.ImageFileID] = append(tagsMap[t.ImageFileID], t.Tag)
	}

	// Build response
	images := make([]smartSearchImageDTO, 0, len(files))
	for _, id := range imageIDs {
		f, ok := fileMap[id]
		if !ok {
			continue
		}

		tagStrs := tagsMap[id]
		if len(tagStrs) > 10 {
			tagStrs = tagStrs[:10]
		}
		sort.Strings(tagStrs)

		images = append(images, smartSearchImageDTO{
			ID:         f.ID,
			Path:       f.Path,
			FileName:   filepath.Base(f.Path),
			ModTime:    f.ModTime.Format("2006-01-02 15:04:05"),
			Similarity: similarityMap[id],
			Tags:       tagStrs,
		})
	}

	c.JSON(http.StatusOK, smartSearchResponse{
		Images: images,
		Total:  len(images),
		Query:  query,
	})
}
