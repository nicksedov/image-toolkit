package imaging

import (
	"fmt"
	"log"
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

	// Query images that need classification
	images, err := om.queryUnclassifiedImages(incremental)
	if err != nil {
		return
	}

	// Update total files count
	om.mu.Lock()
	om.totalFiles = len(images)
	om.mu.Unlock()

	// Handle empty result set
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

	// Log processing plan
	workers := om.GetEffectiveWorkers()
	log.Printf("[OCR] Preparing to process %d images with %d workers", len(images), workers)
	om.updateProgressf("Found %d images to classify", len(images))

	// Launch workers and process results
	om.launchWorkers(images)
}

// launchWorkers creates goroutines for each image and consumes results concurrently
func (om *OcrManager) launchWorkers(images []domain.ImageFile) {
	workers := om.GetEffectiveWorkers()
	stopCheck := om.stopRequestedFunc()

	// Semaphore to limit concurrent OCR requests
	sem := make(chan struct{}, workers)
	results := make(chan OcrResult, len(images))

	var wg sync.WaitGroup
	goroutinesLaunched := 0

	// Launch worker goroutines
loop:
	for i, img := range images {
		// Check for stop request before acquiring semaphore
		if om.isStopRequested() {
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
				if om.isStopRequested() {
					break loop
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
		if !acquired {
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
				<-sem // Release semaphore
				log.Printf("[OCR] Goroutine COMPLETED for image ID=%d", image.ID)
			}()

			log.Printf("[OCR] Goroutine START processing image ID=%d, path=%s", image.ID, image.Path)
			result := om.processSingleImage(image, stopCheck)
			log.Printf("[OCR] Sending result to channel for image ID=%d", image.ID)
			results <- result
		}(img)
	}

	log.Printf("[OCR] All goroutines launched: %d total", goroutinesLaunched)

	// Consume results concurrently with workers
	om.consumeResults(results, &wg)
}

// consumeResults reads from the results channel and saves to database in batches
func (om *OcrManager) consumeResults(results chan OcrResult, wg *sync.WaitGroup) {
	batch := NewClassificationBatch(om.db)
	count := 0

	// Start results consumer goroutine
	log.Printf("[OCR] Starting results consumer goroutine")

	// Wait for workers to finish in a separate goroutine, then close channel
	go func() {
		wg.Wait()
		log.Printf("[OCR] All goroutines completed, closing results channel")
		close(results)
	}()

	// Consumer loop in current goroutine
	log.Printf("[OCR] Results consumer goroutine STARTED")
	for result := range results {
		count++
		if count <= 3 || count%20 == 0 {
			log.Printf("[OCR] Results consumer received result #%d, image ID=%d", count, result.Image.ID)
		}

		// Check for graceful stop request
		if om.isStopRequested() {
			log.Printf("[OCR] Stop requested while processing results at count=%d", count)
			break
		}

		if count%20 == 1 || count <= 5 {
			log.Printf("[OCR] Processing result %d/%d, image ID=%d", count, om.totalFiles, result.Image.ID)
		}

		// Handle errors
		if result.Err != nil {
			log.Printf("[OCR] ERROR processing image ID=%d, path=%s: %v", result.Image.ID, result.Image.Path, result.Err)
			om.updateProgress(count, om.totalFiles, fmt.Sprintf("Error on %s: %v", result.Image.Path, result.Err))
			continue
		}

		// Add successful result to batch
		batch.Add(result)

		// Update progress
		om.updateProgress(count, om.totalFiles, "")

		// Save batch if full
		if batch.IsFull() {
			batch.Save()
		}
	}

	log.Printf("[OCR] Results consumer goroutine EXITED after processing %d results", count)

	// Flush remaining results
	if len(batch.Classifications) > 0 {
		log.Printf("[OCR] Saving final batch of %d classifications", len(batch.Classifications))
		batch.Save()
	}

	// Update final status
	om.mu.Lock()
	if om.stopRequested {
		om.progress = fmt.Sprintf("OCR classification stopped: %d/%d images processed", count, om.totalFiles)
	} else {
		om.progress = fmt.Sprintf("OCR classification complete: %d/%d images processed", count, om.totalFiles)
	}
	log.Printf("[OCR] %s", om.progress)
	om.mu.Unlock()
}
