package thumbnail

import (
	"fmt"
	"os"
	"time"

	"image-toolkit/internal/domain"

	"gorm.io/gorm"
)

// ScanResult holds the result of thumbnail generation during scan
type ScanResult struct {
	ThumbnailCachePath string
	GenerationTimeMs   int64
}

// GenerateThumbnailsDuringScan генерирует миниатюры для новых файлов во время сканирования
func GenerateThumbnailsDuringScan(db *gorm.DB, scanPaths []string, service *Service) error {
	// Get all new image files that don't have thumbnails in cache
	var filesToThumbnail []domain.ImageFile

	for _, scanPath := range scanPaths {
		// Query files in this scan path
		prefix := scanPath + "/"
		db.Where("path LIKE ? AND thumbnail_cache_path = ''", prefix+"%").Find(&filesToThumbnail)
	}

	if len(filesToThumbnail) == 0 {
		return nil
	}

	// Generate thumbnails for all new files
	for _, file := range filesToThumbnail {
		// Check if file exists
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			continue
		}

		// Generate thumbnail
		startTime := time.Now()

		// Generate thumbnail data
		thumbnailData, err := service.GenerateThumbnail(file.Path)
		if err != nil {
			fmt.Printf("Failed to generate thumbnail for %s: %v\n", file.Path, err)
			continue
		}

		// Save to cache using internal storage
		if err := service.saveThumbnailToCache(file.Path, thumbnailData); err != nil {
			fmt.Printf("Failed to save thumbnail for %s: %v\n", file.Path, err)
			continue
		}

		// Update database with cache path
		cachePath := service.GenerateThumbnailPath(file.Path)
		if cachePath != "" {
			db.Model(&file).Update("thumbnail_cache_path", cachePath)
		}

		fmt.Printf("Generated thumbnail for %s (%d ms)\n", file.Path, time.Since(startTime).Milliseconds())
	}

	return nil
}