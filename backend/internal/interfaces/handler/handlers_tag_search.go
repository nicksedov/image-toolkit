package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/interfaces/dto"

	"github.com/gin-gonic/gin"
)

// handleSearchByTags handles GET /api/gallery/tag-search
// Query params: tags=tag1,tag2  matchAll=true|false  page=1  pageSize=20
func (s *Server) handleSearchByTags(c *gin.Context) {
	tagsParam := c.Query("tags")
	if tagsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tags query parameter is required"})
		return
	}

	// Parse comma-separated tags
	rawTags := strings.Split(tagsParam, ",")
	var tags []string
	for _, t := range rawTags {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, strings.ToLower(t))
		}
	}
	if len(tags) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one tag is required"})
		return
	}

	matchAll := c.Query("matchAll") == "true"
	pageSize := 20
	if ps := c.Query("pageSize"); ps != "" {
		if v, err := parseIntDefault(ps, 20); err == nil {
			pageSize = v
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := parseIntDefault(p, 1); err == nil && v > 0 {
			page = v
		}
	}
	offset := (page - 1) * pageSize

	// Build query
	query := s.db.Table("image_files").
		Select("DISTINCT image_files.id, image_files.path, image_files.mod_time").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id")

	if matchAll {
		for _, tag := range tags {
			subQuery := s.db.Table("image_tags").
				Select("image_file_id").
				Where("LOWER(tag) = ?", tag)
			query = query.Where("image_files.id IN (?)", subQuery)
		}
	} else {
		query = query.Where("LOWER(image_tags.tag) IN ?", tags)
	}

	// Count total
	var total int64
	countQuery := s.db.Table("image_files").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id")
	if matchAll {
		for _, tag := range tags {
			subQuery := s.db.Table("image_tags").
				Select("image_file_id").
				Where("LOWER(tag) = ?", tag)
			countQuery = countQuery.Where("image_files.id IN (?)", subQuery)
		}
	} else {
		countQuery = countQuery.Where("LOWER(image_tags.tag) IN ?", tags)
	}
	countQuery.Distinct("image_files.id").Count(&total)

	// Fetch page
	var files []domain.ImageFile
	query.Order("image_files.mod_time DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&files)

	// Build response
	images := make([]dto.TagSearchResult, len(files))
	for i, f := range files {
		images[i] = dto.TagSearchResult{
			ID:       f.ID,
			Path:     f.Path,
			FileName: filepath.Base(f.Path),
			ModTime:  f.ModTime.Format("2006-01-02 15:04:05"),
		}
	}

	s.respondJSON(c, http.StatusOK, dto.TagSearchResponse{
		Images: images,
		Total:  int(total),
	})
}

// parseIntDefault parses a string to int, returning defaultVal on failure.
func parseIntDefault(s string, defaultVal int) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return defaultVal, err
	}
	return v, nil
}
