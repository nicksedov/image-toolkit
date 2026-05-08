package imaging

import (
	"fmt"
	"log"
	"time"

	"image-toolkit/internal/domain"
)

// queryUnclassifiedImages builds and executes a database query to find images that need OCR classification
func (om *OcrManager) queryUnclassifiedImages(incremental bool) ([]domain.ImageFile, error) {
	log.Printf("[OCR] Building query: incremental=%v", incremental)

	query := om.db.Table("image_files").
		Select("image_files.*").
		Joins("LEFT JOIN ocr_classifications ON ocr_classifications.image_file_id = image_files.id")

	if incremental {
		// Only new files (no classification yet) or files modified after last classification
		query = query.Where("ocr_classifications.id IS NULL OR ocr_classifications.updated_at < image_files.updated_at")
	} else {
		// All files: reclassify everything
		query = query.Where("1=1")
	}
	query = query.Order("image_files.id")

	log.Printf("[OCR] Executing database query...")
	queryStart := time.Now()
	var images []domain.ImageFile
	if err := query.Find(&images).Error; err != nil {
		log.Printf("[OCR] ERROR: failed to query images: %v", err)
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	log.Printf("[OCR] Database query completed in %v: found %d images", time.Since(queryStart), len(images))

	return images, nil
}

// updateProgress updates the processing progress status
func (om *OcrManager) updateProgress(filesProcessed, totalFiles int, errorMessage string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	om.filesProcessed = filesProcessed
	if errorMessage != "" {
		om.progress = errorMessage
	} else {
		om.progress = fmt.Sprintf("Classifying: %d/%d", filesProcessed, totalFiles)
	}
}

// updateProgressf updates the processing progress with a formatted message
func (om *OcrManager) updateProgressf(format string, args ...interface{}) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.progress = fmt.Sprintf(format, args...)
}

// isStopRequested checks if a stop has been requested
func (om *OcrManager) isStopRequested() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.stopRequested
}

// stopRequestedFunc returns a function that checks if stop was requested (for passing to workers)
func (om *OcrManager) stopRequestedFunc() func() bool {
	return om.isStopRequested
}
