package imaging

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/ocr"

	"gorm.io/gorm"
)

// OcrManager handles background OCR classification of images
type OcrManager struct {
	mu             sync.RWMutex
	isProcessing   bool
	stopRequested  bool
	incremental    bool
	progress       string
	filesProcessed int
	totalFiles     int
	db             *gorm.DB
	ocrClient      ocr.Client
	maxWorkers     int // Max concurrent OCR requests (0 = auto = runtime.NumCPU())
}

// OcrStatusResponse represents the OCR processing status
type OcrStatusResponse struct {
	Processing     bool   `json:"processing"`
	Incremental    bool   `json:"incremental"`
	Progress       string `json:"progress"`
	FilesProcessed int    `json:"filesProcessed"`
	TotalFiles     int    `json:"totalFiles"`
}

// NewOcrManager creates a new OCR manager
func NewOcrManager(db *gorm.DB, ocrClient ocr.Client, maxWorkers int) *OcrManager {
	return &OcrManager{
		db:         db,
		ocrClient:  ocrClient,
		maxWorkers: maxWorkers,
	}
}

// StartClassification starts the OCR classification process in background
func (om *OcrManager) StartClassification(incremental bool) error {
	om.mu.Lock()
	if om.isProcessing {
		om.mu.Unlock()
		return fmt.Errorf("OCR classification already in progress")
	}
	om.isProcessing = true
	om.stopRequested = false
	om.incremental = incremental
	if incremental {
		om.progress = "Starting OCR classification (changes only)..."
	} else {
		om.progress = "Starting OCR classification..."
	}
	om.filesProcessed = 0
	om.totalFiles = 0
	om.mu.Unlock()

	go om.processUnclassified(incremental)

	return nil
}

// GetStatus returns the current OCR processing status
func (om *OcrManager) GetStatus() OcrStatusResponse {
	om.mu.RLock()
	defer om.mu.RUnlock()

	return OcrStatusResponse{
		Processing:     om.isProcessing,
		Incremental:    om.incremental,
		Progress:       om.progress,
		FilesProcessed: om.filesProcessed,
		TotalFiles:     om.totalFiles,
	}
}

// IsProcessing returns true if OCR classification is currently running
func (om *OcrManager) IsProcessing() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.isProcessing
}

// StopClassification requests a graceful stop of the current OCR classification
func (om *OcrManager) StopClassification() {
	om.mu.Lock()
	defer om.mu.Unlock()
	if om.isProcessing {
		om.stopRequested = true
		om.progress = "Stopping OCR classification..."
	}
}

// SetMaxWorkers updates the maximum number of concurrent OCR workers.
// This takes effect immediately for ongoing classification as well.
func (om *OcrManager) SetMaxWorkers(workers int) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.maxWorkers = workers
}

// GetEffectiveWorkers returns the effective number of workers.
// If maxWorkers is 0, returns runtime.NumCPU() (capped at NumCPU).
func (om *OcrManager) GetEffectiveWorkers() int {
	om.mu.RLock()
	defer om.mu.RUnlock()
	if om.maxWorkers <= 0 {
		return runtime.NumCPU()
	}
	// Cap at NumCPU if exceeding
	if om.maxWorkers > runtime.NumCPU() {
		return runtime.NumCPU()
	}
	return om.maxWorkers
}

// processUnclassified finds images without OCR classification and processes them
func (om *OcrManager) processUnclassified(incremental bool) {
	log.Printf("[OCR] Starting processUnclassified: incremental=%v, maxWorkers=%d", incremental, om.maxWorkers)

	defer func() {
		om.mu.Lock()
		om.isProcessing = false
		if om.stopRequested {
			om.progress = "OCR classification stopped"
		} else {
			om.progress = "OCR classification complete"
		}
		log.Printf("[OCR] processUnclassified finished: isProcessing=false, stopRequested=%v", om.stopRequested)
		om.mu.Unlock()
	}()

	// Build query based on mode
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
		return
	}
	log.Printf("[OCR] Database query completed in %v: found %d images", time.Since(queryStart), len(images))

	om.mu.Lock()
	om.totalFiles = len(images)
	om.mu.Unlock()

	if len(images) == 0 {
		log.Printf("[OCR] No images to process")
		om.mu.Lock()
		if incremental {
			om.progress = "No new or changed images found"
		} else {
			om.progress = "No images found"
		}
		om.isProcessing = false
		om.mu.Unlock()
		return
	}

	workers := om.GetEffectiveWorkers()
	log.Printf("[OCR] Preparing to process %d images with %d workers", len(images), workers)
	om.mu.Lock()
	om.progress = fmt.Sprintf("Found %d images to classify", len(images))
	om.mu.Unlock()

	// Process images with limited concurrency using semaphore
	type ocrResult struct {
		image          domain.ImageFile
		classification *domain.OcrClassification
		boxes          []domain.OcrBoundingBox
		err            error
	}

	// Create semaphore to limit concurrent OCR requests
	sem := make(chan struct{}, workers)
	results := make(chan ocrResult, len(images))

	var wg sync.WaitGroup
	goroutinesLaunched := 0
loop:
	for i, img := range images {
		// Check for stop request before acquiring semaphore
		om.mu.RLock()
		stop := om.stopRequested
		om.mu.RUnlock()
		if stop {
			break
		}

		// Acquire semaphore with stop check
		acquired := false
		for !acquired {
			select {
			case sem <- struct{}{}:
				acquired = true
			default:
				// Check if we should stop while waiting
				om.mu.RLock()
				stop = om.stopRequested
				om.mu.RUnlock()
				if stop {
					break loop // break out of the outer for loop
				}
				// Brief sleep before retry (50ms)
				time.Sleep(50 * time.Millisecond)
			}
		}
		if stop {
			break
		}

		wg.Add(1)
		goroutinesLaunched++
		if (i+1)%100 == 0 || i < 5 {
			log.Printf("[OCR] Launching goroutine %d/%d for image ID=%d", i+1, len(images), img.ID)
		}
		go func(image domain.ImageFile) {
			defer wg.Done()
			defer func() {
				<-sem
				log.Printf("[OCR] Goroutine COMPLETED for image ID=%d", image.ID)
			}() // Release semaphore

			log.Printf("[OCR] Goroutine START processing image ID=%d, path=%s", image.ID, image.Path)
			result := ocrResult{image: image}

			// Check for stop before processing
			om.mu.RLock()
			stopNow := om.stopRequested
			om.mu.RUnlock()
			if stopNow {
				log.Printf("[OCR] Image ID=%d STOPPED before processing", image.ID)
				result.err = fmt.Errorf("classification stopped")
				results <- result
				return
			}

			// Open image file
			log.Printf("[OCR] Opening file: %s (image ID=%d)", image.Path, image.ID)
			file, err := os.Open(image.Path)
			if err != nil {
				log.Printf("[OCR] ERROR opening file %s (ID=%d): %v", image.Path, image.ID, err)
				result.err = fmt.Errorf("failed to open file: %w", err)
				results <- result
				return
			}
			log.Printf("[OCR] File opened OK: %s (ID=%d)", image.Path, image.ID)

			// Determine content type based on extension
			contentType := "image/jpeg"
			if len(image.Path) > 4 {
				ext := image.Path[len(image.Path)-4:]
				if ext == ".png" {
					contentType = "image/png"
				}
			}

			// Call OCR API
			log.Printf("[OCR] Calling OCR API for image ID=%d, path=%s, contentType=%s", image.ID, image.Path, contentType)
			ocrStart := time.Now()
			ctx := context.Background()
			ocrResp, err := om.ocrClient.Classify(ctx, file, contentType, ocr.DefaultClassifyParams())
			ocrDuration := time.Since(ocrStart)
			file.Close()

			if err != nil {
				log.Printf("[OCR] OCR API FAILED for image ID=%d after %v: %v", image.ID, ocrDuration, err)
				result.err = fmt.Errorf("OCR classification failed: %w", err)
				results <- result
				return
			}
			log.Printf("[OCR] OCR API OK for image ID=%d in %v: isText=%v, confidence=%.3f", image.ID, ocrDuration, ocrResp.IsTextDocument, ocrResp.MeanConfidence)

			// Create classification record
			classification := &domain.OcrClassification{
				ImageFileID:        image.ID,
				IsTextDocument:     ocrResp.IsTextDocument,
				MeanConfidence:     ocrResp.MeanConfidence,
				WeightedConfidence: ocrResp.WeightedConfidence,
				TokenCount:         ocrResp.TokenCount,
				Angle:              ocrResp.Angle,
				ScaleFactor:        ocrResp.ScaleFactor,
				BoundingBoxWidth:   ocrResp.BoundingBoxWidth,
				BoundingBoxHeight:  ocrResp.BoundingBoxHeight,
			}

			// Create bounding box records only for text documents
			var boxes []domain.OcrBoundingBox
			if ocrResp.IsTextDocument && len(ocrResp.Boxes) > 0 {
				for _, box := range ocrResp.Boxes {
					boxes = append(boxes, domain.OcrBoundingBox{
						X:          box.X,
						Y:          box.Y,
						Width:      box.Width,
						Height:     box.Height,
						Word:       box.Word,
						Confidence: box.Confidence,
					})
				}
			}

			result.classification = classification
			result.boxes = boxes
			log.Printf("[OCR] Sending result to channel for image ID=%d", image.ID)
			results <- result
		}(img)
	}

	log.Printf("[OCR] All goroutines launched: %d total, waiting for results...", goroutinesLaunched)

	// Process results with batch DB writes (runs concurrently with goroutines)
	const batchSize = 20
	var toCreate []domain.OcrClassification
	// Map to track boxes by image file ID
	boxesByImage := make(map[uint][]domain.OcrBoundingBox)

	// Start results consumer in a separate goroutine
	log.Printf("[OCR] Starting results consumer goroutine")
	go func() {
		log.Printf("[OCR] Results consumer goroutine STARTED")
		count := 0
		for result := range results {
			count++
			if count <= 3 || count%20 == 0 {
				log.Printf("[OCR] Results consumer received result #%d, image ID=%d", count, result.image.ID)
			}

			// Check for graceful stop request
			om.mu.RLock()
			stop := om.stopRequested
			om.mu.RUnlock()
			if stop {
				log.Printf("[OCR] Stop requested while processing results at count=%d", count)
				break
			}

			if count%20 == 1 || count <= 5 {
				log.Printf("[OCR] Processing result %d/%d, image ID=%d", count, om.totalFiles, result.image.ID)
			}

			if result.err != nil {
				log.Printf("[OCR] ERROR processing image ID=%d, path=%s: %v", result.image.ID, result.image.Path, result.err)
				om.mu.Lock()
				om.filesProcessed = count
				om.progress = fmt.Sprintf("Error on %s: %v", result.image.Path, result.err)
				om.mu.Unlock()
				continue
			}

			if result.classification != nil {
				toCreate = append(toCreate, *result.classification)
				// Store boxes keyed by image file ID for later lookup
				if len(result.boxes) > 0 {
					boxesByImage[result.classification.ImageFileID] = result.boxes
				}
			}

			om.mu.Lock()
			om.filesProcessed = count
			om.progress = fmt.Sprintf("Classifying: %d/%d", count, om.totalFiles)
			om.mu.Unlock()

			// Batch write classifications
			if len(toCreate) >= batchSize {
				log.Printf("[OCR] Saving batch of %d classifications (count=%d)", len(toCreate), count)
				om.saveClassificationBatch(&toCreate, boxesByImage)
				boxesByImage = make(map[uint][]domain.OcrBoundingBox)
			}
		}

		log.Printf("[OCR] Results consumer goroutine EXITED after processing %d results", count)

		// Flush remaining
		if len(toCreate) > 0 {
			log.Printf("[OCR] Saving final batch of %d classifications", len(toCreate))
			om.saveClassificationBatch(&toCreate, boxesByImage)
		}

		om.mu.Lock()
		if om.stopRequested {
			om.progress = fmt.Sprintf("OCR classification stopped: %d/%d images processed", count, om.totalFiles)
			log.Printf("[OCR] %s", om.progress)
		} else {
			om.progress = fmt.Sprintf("OCR classification complete: %d/%d images processed", count, om.totalFiles)
			log.Printf("[OCR] %s", om.progress)
		}
		om.mu.Unlock()
	}()

	// Wait for all goroutines to finish, then close results channel
	wg.Wait()
	log.Printf("[OCR] All goroutines completed, closing results channel")
	close(results)
}

// saveClassificationBatch saves a batch of classifications and their bounding boxes
func (om *OcrManager) saveClassificationBatch(classifications *[]domain.OcrClassification, boxesByImage map[uint][]domain.OcrBoundingBox) {
	if len(*classifications) == 0 {
		*classifications = (*classifications)[:0]
		return
	}

	// Save each classification and its bounding boxes
	for i := range *classifications {
		classification := &(*classifications)[i]
		if err := om.db.Where("image_file_id = ?", classification.ImageFileID).Assign(classification).FirstOrCreate(classification).Error; err != nil {
			log.Printf("OCR: failed to save classification for image %d: %v", classification.ImageFileID, err)
			continue
		}

		// Save bounding boxes only for text document classifications
		if classification.IsTextDocument {
			if boxes, ok := boxesByImage[classification.ImageFileID]; ok {
				// Delete old bounding boxes for this classification before inserting new ones
				om.db.Where("classification_id = ?", classification.ID).Delete(&domain.OcrBoundingBox{})

				for j := range boxes {
					boxes[j].ClassificationID = classification.ID
					if err := om.db.Create(&boxes[j]).Error; err != nil {
						log.Printf("OCR: failed to save bounding box for classification %d: %v", classification.ID, err)
					}
				}
				// Clean up to avoid re-processing
				delete(boxesByImage, classification.ImageFileID)
			}
		}
	}

	*classifications = (*classifications)[:0]
}
