package handler

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"image-toolkit/internal/application/geo"
	"image-toolkit/internal/application/imaging"
	"image-toolkit/internal/application/thumbnail"
	"image-toolkit/internal/infrastructure/config"
	"image-toolkit/internal/infrastructure/ocr"
	"image-toolkit/internal/interfaces/dto"
	"image-toolkit/internal/interfaces/handler/helpers"

	"gorm.io/gorm"
)

// Server holds the application state
type Server struct {
	db               *gorm.DB
	thumbnailCache   *imaging.ThumbnailCache
	thumbnailService *thumbnail.Service
	thumbnailBatch   *helpers.ThumbnailBatch
	scanManager      *imaging.ScanManager
	ocrManager       *imaging.OcrManager
	llmOcrService    *imaging.LlmOcrService
	backgroundSync   *imaging.BackgroundSyncManager
	config           *config.AppConfig
	ocrClient        ocr.Client
	clusterStorage   *geo.ClusterStorage
	galleryAccess    *helpers.GalleryAccess
	settingsLoader   *helpers.SettingsLoader
	llmFactory       *helpers.LLMFactory
	fileMover        *helpers.FileMover
}

// NewServer creates a new server instance
func NewServer(db *gorm.DB, scanManager *imaging.ScanManager, ocrManager *imaging.OcrManager, llmOcrService *imaging.LlmOcrService, backgroundSync *imaging.BackgroundSyncManager, thumbnailService *thumbnail.Service, cfg *config.AppConfig) *Server {
	var ocrClient ocr.Client
	if cfg.OCREnabled {
		ocrClient = ocr.NewClient(cfg.OCRHost, cfg.OCRPort)
	}
	s := &Server{
		db:               db,
		thumbnailCache:   imaging.NewThumbnailCache(),
		thumbnailService: thumbnailService,
		scanManager:      scanManager,
		ocrManager:       ocrManager,
		llmOcrService:    llmOcrService,
		backgroundSync:   backgroundSync,
		config:           cfg,
		ocrClient:        ocrClient,
		clusterStorage:   geo.NewClusterStorage(),
	}
	s.thumbnailBatch = helpers.NewThumbnailBatch(thumbnailService, s.thumbnailCache)
	s.galleryAccess = helpers.NewGalleryAccess(db)
	s.settingsLoader = helpers.NewSettingsLoader(db)
	s.llmFactory = helpers.NewLLMFactory(db)
	s.fileMover = helpers.NewFileMover(db)
	return s
}

// StartOCRHealthCheck starts the OCR health check in background
func (s *Server) StartOCRHealthCheck() {
	if s.ocrClient != nil && s.config.OCREnabled {
		s.ocrClient.StartHealthCheck(s.config.OCRCheckInterval)
	}
}

// StopOCRHealthCheck stops the OCR health check
func (s *Server) StopOCRHealthCheck() {
	if s.ocrClient != nil {
		s.ocrClient.StopHealthCheck()
	}
}

// formatSize formats file size in human readable format
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// pathsConflict checks if two normalized (forward-slash) paths are the same,
// or if one is a parent/child of the other.
// Returns a non-empty reason string if there is a conflict, empty string otherwise.
func pathsConflict(a, b string) string {
	// Normalize: trim trailing slashes, lowercase for case-insensitive FS
	na := strings.TrimRight(strings.ToLower(a), "/")
	nb := strings.TrimRight(strings.ToLower(b), "/")

	if na == nb {
		return "same"
	}
	if strings.HasPrefix(na, nb+"/") {
		return "child" // a is child of b
	}
	if strings.HasPrefix(nb, na+"/") {
		return "parent" // a is parent of b
	}
	return ""
}

// sortPatternsByCount sorts patterns by duplicate count descending
func sortPatternsByCount(patterns []dto.FolderPattern) {
	slices.SortFunc(patterns, func(a, b dto.FolderPattern) int {
		return cmp.Compare(b.DuplicateCount, a.DuplicateCount)
	})
}

// createPatternID creates a unique ID from sorted folder paths
func createPatternID(folders []string) string {
	return strings.Join(folders, "|")
}
