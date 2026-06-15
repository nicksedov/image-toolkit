package imaging

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"image-toolkit/internal/application/thumbnail"
	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/geocoder"

	"gorm.io/gorm"
)

// BackgroundSyncManager manages background synchronization of gallery files
// and thumbnail cache. It runs daily at a configured time.
type BackgroundSyncManager struct {
	mu                 sync.Mutex
	running            bool
	stopCh             chan struct{}
	scheduleCh         chan struct{} // Signal to restart the schedule loop
	db                 *gorm.DB
	thumbnailService   *thumbnail.Service
	geolocationService *geocoder.GeolocationService
	enabled            bool
	hour               int
	minute             int
}

// NewBackgroundSyncManager creates a new background sync manager
func NewBackgroundSyncManager(db *gorm.DB, thumbnailService *thumbnail.Service, geoService *geocoder.GeolocationService) *BackgroundSyncManager {
	return &BackgroundSyncManager{
		db:                 db,
		thumbnailService:   thumbnailService,
		geolocationService: geoService,
		enabled:            true,
		hour:               3,
		minute:             30,
		stopCh:             make(chan struct{}),
		scheduleCh:         make(chan struct{}),
	}
}

// Start begins the background synchronization loop with the given schedule
func (bsm *BackgroundSyncManager) Start(enabled bool, hour int, minute int) {
	bsm.mu.Lock()
	if bsm.running {
		bsm.mu.Unlock()
		log.Println("Background sync already running")
		return
	}
	bsm.running = true
	bsm.enabled = enabled
	bsm.hour = hour
	bsm.minute = minute
	bsm.stopCh = make(chan struct{})
	bsm.scheduleCh = make(chan struct{})
	bsm.mu.Unlock()

	log.Printf("Starting background gallery sync (daily at %02d:%02d, enabled=%v)", hour, minute, enabled)
	go bsm.scheduleLoop()
}

// Stop stops the background synchronization
func (bsm *BackgroundSyncManager) Stop() {
	bsm.mu.Lock()
	if !bsm.running {
		bsm.mu.Unlock()
		return
	}
	bsm.running = false
	close(bsm.stopCh)
	bsm.mu.Unlock()

	log.Println("Background gallery sync stopped")
}

// UpdateSchedule updates the schedule at runtime and restarts the loop
func (bsm *BackgroundSyncManager) UpdateSchedule(enabled bool, hour int, minute int) {
	bsm.mu.Lock()
	wasRunning := bsm.running
	bsm.enabled = enabled
	bsm.hour = hour
	bsm.minute = minute
	bsm.mu.Unlock()

	log.Printf("Background sync schedule updated: daily at %02d:%02d, enabled=%v", hour, minute, enabled)

	// Signal the schedule loop to restart with new settings
	if wasRunning {
		select {
		case bsm.scheduleCh <- struct{}{}:
		default:
		}
	}
}

// scheduleLoop runs the synchronization daily at the configured time
func (bsm *BackgroundSyncManager) scheduleLoop() {
	for {
		bsm.mu.Lock()
		enabled := bsm.enabled
		hour := bsm.hour
		minute := bsm.minute
		stopCh := bsm.stopCh
		bsm.mu.Unlock()

		if enabled {
			// Calculate time until next run
			nextRun := bsm.calculateNextRunTime(hour, minute)
			log.Printf("Background sync: next run at %s", nextRun.Format("15:04:05"))

			select {
			case <-time.After(time.Until(nextRun)):
				// Time to run sync
				bsm.syncOnce()
			case <-stopCh:
				return
			case <-bsm.scheduleCh:
				// Schedule updated, restart the loop
				continue
			}
		} else {
			log.Println("Background sync: disabled, waiting for schedule change or stop")
			select {
			case <-stopCh:
				return
			case <-bsm.scheduleCh:
				// Schedule updated, restart the loop
				continue
			}
		}
	}
}

// calculateNextRunTime calculates the next time the sync should run
func (bsm *BackgroundSyncManager) calculateNextRunTime(hour, minute int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	// If the time has already passed today, schedule for tomorrow
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	return next
}

// syncOnce performs a single synchronization pass
func (bsm *BackgroundSyncManager) syncOnce() {
	log.Println("Background sync: starting gallery synchronization")

	// Get all gallery folders
	var folders []domain.GalleryFolder
	if err := bsm.db.Find(&folders).Error; err != nil {
		log.Printf("Background sync: failed to get gallery folders: %v", err)
		return
	}

	if len(folders) == 0 {
		log.Println("Background sync: no gallery folders configured")
		return
	}

	// Check if thumbnail service is available
	thumbnailEnabled := bsm.thumbnailService != nil && bsm.thumbnailService.IsEnabled()

	newFiles := 0
	updatedFiles := 0
	deletedFiles := 0
	thumbnailGenerated := 0

	for _, folder := range folders {
		absPath, err := filepath.Abs(folder.Path)
		if err != nil {
			log.Printf("Background sync: failed to get absolute path for %s: %v", folder.Path, err)
			continue
		}

		folderNew, folderUpdated, folderDeleted, folderThumbGenerated := bsm.syncFolder(absPath, thumbnailEnabled)
		newFiles += folderNew
		updatedFiles += folderUpdated
		deletedFiles += folderDeleted
		thumbnailGenerated += folderThumbGenerated
	}

	// Clean up records for files that no longer exist
	deletedFiles += bsm.cleanupMissingFiles()

	log.Printf("Background sync: complete - %d new, %d updated, %d deleted, %d thumbnails generated",
		newFiles, updatedFiles, deletedFiles, thumbnailGenerated)
}

// syncFolder synchronizes a single folder sequentially
func (bsm *BackgroundSyncManager) syncFolder(folderPath string, thumbnailEnabled bool) (newCount, updatedCount, deletedCount, thumbCount int) {
	log.Printf("Background sync: scanning folder %s", folderPath)

	// Collect all image files from disk
	diskFiles := make(map[string]os.FileInfo)
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Background sync: error accessing %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !domain.IsImageFile(path) {
			return nil
		}
		normalizedPath := filepath.ToSlash(path)
		diskFiles[normalizedPath] = info
		return nil
	})

	if err != nil {
		log.Printf("Background sync: failed to walk folder %s: %v", folderPath, err)
		return
	}

	// Get all existing DB records for this folder
	var dbFiles []domain.ImageFile
	prefix := folderPath + "/"
	if err := bsm.db.Where("path LIKE ?", prefix+"%").Find(&dbFiles).Error; err != nil {
		log.Printf("Background sync: failed to query DB for folder %s: %v", folderPath, err)
		return
	}

	// Build a map of DB files
	dbFileMap := make(map[string]domain.ImageFile)
	for _, dbFile := range dbFiles {
		dbFileMap[dbFile.Path] = dbFile
	}

	// Process each file on disk sequentially
	for diskPath, diskInfo := range diskFiles {
		// Check if we should stop
		if !bsm.isRunning() {
			log.Println("Background sync: stopped during folder scan")
			return
		}

		dbFile, existsInDB := dbFileMap[diskPath]

		if !existsInDB {
			// New file - add to DB
			hash, err := calculateFileHash(diskPath)
			if err != nil {
				log.Printf("Background sync: failed to hash new file %s: %v", diskPath, err)
				continue
			}

			newFile := domain.ImageFile{
				Path:    diskPath,
				Size:    diskInfo.Size(),
				Hash:    hash,
				ModTime: diskInfo.ModTime(),
			}

			if err := bsm.db.Create(&newFile).Error; err != nil {
				log.Printf("Background sync: failed to create record for %s: %v", diskPath, err)
				continue
			}

			newCount++
			log.Printf("Background sync: added new file %s", diskPath)

			// Extract EXIF/geo metadata for new file
			bsm.ExtractAndSaveMetadata(diskPath, newFile.ID)

			// Generate thumbnail for new file
			if thumbnailEnabled {
				if bsm.ensureThumbnail(diskPath) {
					thumbCount++
				}
			}
		} else {
			// File exists in DB - check if modified
			sizeChanged := dbFile.Size != diskInfo.Size()
			modTimeChanged := !dbFile.ModTime.Equal(diskInfo.ModTime())

			if sizeChanged || modTimeChanged {
				hash, err := calculateFileHash(diskPath)
				if err != nil {
					log.Printf("Background sync: failed to hash modified file %s: %v", diskPath, err)
					continue
				}

				hashChanged := dbFile.Hash != hash
				contentChanged := sizeChanged || hashChanged
				dbFile.Size = diskInfo.Size()
				dbFile.Hash = hash
				dbFile.ModTime = diskInfo.ModTime()

				if err := bsm.db.Save(&dbFile).Error; err != nil {
					log.Printf("Background sync: failed to update record for %s: %v", diskPath, err)
					continue
				}

				updatedCount++
				log.Printf("Background sync: updated file %s (size:%v, modtime:%v, hash:%v)", diskPath, sizeChanged, modTimeChanged, hashChanged)

				// Re-extract EXIF/geo metadata only if file content actually changed
				if contentChanged {
					bsm.ExtractAndSaveMetadata(diskPath, dbFile.ID)
				}

				// Invalidate OCR classification only if file content actually changed
				if contentChanged {
					bsm.InvalidateOCRClassification(dbFile.ID)
				}

				// Invalidate AI tags and embeddings only if file content actually changed
				if contentChanged {
					bsm.InvalidateTagsAndEmbeddings(dbFile.ID)
				}

				// Regenerate thumbnail for modified file (invalidate old one)
				if thumbnailEnabled && contentChanged {
					bsm.thumbnailService.Invalidate(diskPath)
					if bsm.ensureThumbnail(diskPath) {
						thumbCount++
					}
				}
			} else {
				// File unchanged - ensure thumbnail exists
				if thumbnailEnabled && !bsm.thumbnailService.HasThumbnail(diskPath) {
					if bsm.ensureThumbnail(diskPath) {
						thumbCount++
					}
				}
			}
		}
	}

	return
}

// ensureThumbnail generates a thumbnail for a file if it doesn't exist
func (bsm *BackgroundSyncManager) ensureThumbnail(filePath string) bool {
	// Check if file still exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	// Check if thumbnail already exists
	if bsm.thumbnailService.HasThumbnail(filePath) {
		return false
	}

	// Generate thumbnail
	_, err := bsm.thumbnailService.GetOrGenerate(filePath)
	if err != nil {
		log.Printf("Background sync: failed to generate thumbnail for %s: %v", filePath, err)
		return false
	}

	log.Printf("Background sync: generated thumbnail for %s", filePath)
	return true
}

// cleanupMissingFiles removes DB records for files that no longer exist on disk
func (bsm *BackgroundSyncManager) cleanupMissingFiles() int {
	var files []domain.ImageFile
	if err := bsm.db.Find(&files).Error; err != nil {
		log.Printf("Background sync: failed to query all files for cleanup: %v", err)
		return 0
	}

	deletedCount := 0
	for _, file := range files {
		if !bsm.isRunning() {
			log.Println("Background sync: stopped during cleanup")
			break
		}

		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			deleteImageFileCascade(bsm.db, file.ID)

			// Clean up thumbnail if it exists
			if bsm.thumbnailService != nil {
				bsm.thumbnailService.Invalidate(file.Path)
			}

			deletedCount++
			log.Printf("Background sync: removed missing file record %s", file.Path)
		}
	}

	return deletedCount
}

// isRunning checks if the sync manager is currently running
func (bsm *BackgroundSyncManager) isRunning() bool {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()
	return bsm.running
}

// ExtractAndSaveMetadata extracts EXIF and geo metadata for a file and saves to the database.
func (bsm *BackgroundSyncManager) ExtractAndSaveMetadata(filePath string, imageFileID uint) {
	meta, err := extractMetadata(filePath)
	if err != nil {
		log.Printf("Background sync: failed to extract metadata for %s: %v", filePath, err)
		return
	}

	meta.ImageFileID = imageFileID

	// Resolve geolocation via GeolocationService if GPS coordinates are present.
	if bsm.geolocationService != nil {
		lat, lng, hasGPS := extractGPSCoordinates(filePath)
		if hasGPS {
			geoEntry, err := bsm.geolocationService.ResolveGeolocation(lat, lng)
			if err != nil {
				log.Printf("Background sync: failed to resolve geolocation for %s: %v", filePath, err)
			} else {
				meta.GeolocationRef = &geoEntry.ID
			}
		}
	}

	// Upsert: insert or update
	if err := bsm.db.Where("image_file_id = ?", imageFileID).Assign(meta).FirstOrCreate(&domain.ImageMetadata{}).Error; err != nil {
		log.Printf("Background sync: failed to save metadata for %s: %v", filePath, err)
	} else {
		log.Printf("Background sync: saved EXIF/geo metadata for %s", filePath)
	}
}

// InvalidateOCRClassification deletes existing OCR classification for a file
// so it gets re-classified on the next OCR pass.
func (bsm *BackgroundSyncManager) InvalidateOCRClassification(imageFileID uint) {
	// Delete bounding boxes first (foreign key dependency)
	bsm.db.Where("classification_id IN (SELECT id FROM ocr_classifications WHERE image_file_id = ?)", imageFileID).Delete(&domain.OcrBoundingBox{})
	// Delete the classification
	if result := bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.OcrClassification{}); result.Error == nil && result.RowsAffected > 0 {
		log.Printf("Background sync: invalidated OCR classification for image %d", imageFileID)
	}
}

// ExtractAndSaveMetadataAsync extracts EXIF/geo metadata in a background goroutine.
func (bsm *BackgroundSyncManager) ExtractAndSaveMetadataAsync(filePath string, imageFileID uint) {
	go bsm.ExtractAndSaveMetadata(filePath, imageFileID)
}

// InvalidateOCRClassificationAsync invalidates OCR classification in a background goroutine.
func (bsm *BackgroundSyncManager) InvalidateOCRClassificationAsync(imageFileID uint) {
	go bsm.InvalidateOCRClassification(imageFileID)
}

// InvalidateTagsAndEmbeddingsAsync invalidates AI tags and embeddings in a background goroutine.
func (bsm *BackgroundSyncManager) InvalidateTagsAndEmbeddingsAsync(imageFileID uint) {
	go bsm.InvalidateTagsAndEmbeddings(imageFileID)
}

// invalidateTagsAndEmbeddings deletes AI-generated tags and vector embeddings for a file
// so they are re-generated on the next tag scan or embedding backfill pass.
func (bsm *BackgroundSyncManager) InvalidateTagsAndEmbeddings(imageFileID uint) {
	bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageTag{})
	if result := bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.TagEmbedding{}); result.Error == nil && result.RowsAffected > 0 {
		log.Printf("Background sync: invalidated tags and embeddings for image %d", imageFileID)
	}
}

