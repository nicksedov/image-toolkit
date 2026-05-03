package handler

import (
	"net/http"

	"image-toolkit/internal/application/thumbnail"
	"image-toolkit/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// ThumbnailService interface для инъекции зависимости
type ThumbnailService interface {
	Start() error
	Stop()
	IsEnabled() bool
	GetThumbnailPath(filePath string) string
	HasThumbnail(filePath string) bool
	GetOrGenerate(filePath string) (string, error)
	GenerateThumbnail(filePath string) ([]byte, error)
	Invalidate(filePath string) error
	InvalidateAll() error
	Warmup(imagePaths []string) error
	Stats() thumbnail.ThumbnailStats
	UpdateCachePath(newPath string) error
	Enable()
	Disable()
}

// thumbnailHandler обработчики для кэша миниатюр
type thumbnailHandler struct {
	service ThumbnailService
}

// NewThumbnailHandler создает новый thumbnailHandler
func NewThumbnailHandler(service ThumbnailService) *thumbnailHandler {
	return &thumbnailHandler{service: service}
}

// handleThumbnailCacheStats возвращает статистику кэша миниатюр
// GET /api/thumbnail/cache/stats
func (th *thumbnailHandler) handleThumbnailCacheStats(c *gin.Context) {
	stats := th.service.Stats()
	c.JSON(http.StatusOK, stats)
}

// handleThumbnailCacheInvalidate удаляет миниатюру из кэша
// DELETE /api/thumbnail/cache/invalidate
func (th *thumbnailHandler) handleThumbnailCacheInvalidate(c *gin.Context) {
	var req InvalidateThumbnailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	if err := th.service.Invalidate(req.FilePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "thumbnail invalidated"})
}

// handleThumbnailCacheInvalidateAll удаляет все миниатюры из кэша
// DELETE /api/thumbnail/cache/invalidate-all
func (th *thumbnailHandler) handleThumbnailCacheInvalidateAll(c *gin.Context) {
	if err := th.service.InvalidateAll(); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "all thumbnails invalidated"})
}

// handleThumbnailCacheWarmup предварительно генерирует миниатюры для файлов
// POST /api/thumbnail/cache/warmup
func (th *thumbnailHandler) handleThumbnailCacheWarmup(c *gin.Context) {
	var req WarmupThumbnailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgScanNoFilesSelected))
		return
	}

	if err := th.service.Warmup(req.FilePaths); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "thumbnails warmed up"})
}

// handleThumbnailCacheEnable включает кэш миниатюр
// POST /api/thumbnail/cache/enable
func (th *thumbnailHandler) handleThumbnailCacheEnable(c *gin.Context) {
	th.service.Enable()
	c.JSON(http.StatusOK, gin.H{"message": "thumbnail cache enabled"})
}

// handleThumbnailCacheDisable выключает кэш миниатюр
// POST /api/thumbnail/cache/disable
func (th *thumbnailHandler) handleThumbnailCacheDisable(c *gin.Context) {
	th.service.Disable()
	c.JSON(http.StatusOK, gin.H{"message": "thumbnail cache disabled"})
}

// InvalidateThumbnailRequest запрос на удаление миниатюры
type InvalidateThumbnailRequest struct {
	FilePath string `json:"filePath" binding:"required"`
}

// WarmupThumbnailsRequest запрос на предварительную генерацию миниатюр
type WarmupThumbnailsRequest struct {
	FilePaths []string `json:"filePaths" binding:"required"`
}
