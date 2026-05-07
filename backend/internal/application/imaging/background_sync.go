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
// and thumbnail cache. It runs sequentially, processing one file at a time.
type BackgroundSyncManager struct {
	mu               sync.Mutex
	running          bool
	stopCh           chan struct{}
	db               *gorm.DB
	thumbnailService *thumbnail.Service
	geocoder         *geocoder.Geocoder
	syncInterval     time.Duration
}

// NewBackgroundSyncManager creates a new background sync manager
func NewBackgroundSyncManager(db *gorm.DB, thumbnailService *thumbnail.Service, geo *geocoder.Geocoder, syncIntervalMinutes int) *BackgroundSyncManager {
	interval := time.Duration(syncIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 12 * 60 * time.Minute
	}

	return &BackgroundSyncManager{
		db:               db,
		thumbnailService: thumbnailService,
		geocoder:         geo,
		syncInterval:     interval,
		stopCh:           make(chan struct{}),
	}
}

// Start begins the background synchronization loop
func (bsm *BackgroundSyncManager) Start() {
	bsm.mu.Lock()
	if bsm.running {
		bsm.mu.Unlock()
		log.Println("Background sync already running")
		return
	}
	bsm.running = true
	bsm.stopCh = make(chan struct{})
	bsm.mu.Unlock()

	log.Printf("Starting background gallery sync (interval: %v)", bsm.syncInterval)
	go bsm.syncLoop()
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

// IsRunning returns whether the background sync is currently running
func (bsm *BackgroundSyncManager) IsRunning() bool {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()
	return bsm.running
}

// syncLoop runs the synchronization at configured intervals
func (bsm *BackgroundSyncManager) syncLoop() {
	// Run immediately on start
	bsm.syncOnce()

	ticker := time.NewTicker(bsm.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bsm.syncOnce()
		case <-bsm.stopCh:
			return
		}
	}
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
			bsm.extractAndSaveMetadata(diskPath, newFile.ID)

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
				dbFile.Size = diskInfo.Size()
				dbFile.Hash = hash
				dbFile.ModTime = diskInfo.ModTime()

				if err := bsm.db.Save(&dbFile).Error; err != nil {
					log.Printf("Background sync: failed to update record for %s: %v", diskPath, err)
					continue
				}

				updatedCount++
				log.Printf("Background sync: updated file %s (size:%v, modtime:%v, hash:%v)", diskPath, sizeChanged, modTimeChanged, hashChanged)

				// Re-extract EXIF/geo metadata for modified file
				bsm.extractAndSaveMetadata(diskPath, dbFile.ID)

				// Invalidate OCR classification only if file content actually changed
				// Size change = definite content change; ModTime change alone requires hash comparison
				if sizeChanged || hashChanged {
					bsm.invalidateOCRClassification(dbFile.ID)
				}

				// Regenerate thumbnail for modified file (invalidate old one)
				if thumbnailEnabled && (sizeChanged || hashChanged) {
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
			if err := bsm.db.Delete(&file).Error; err != nil {
				log.Printf("Background sync: failed to delete record for missing file %s: %v", file.Path, err)
				continue
			}

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

// extractAndSaveMetadata extracts EXIF and geo metadata for a file and saves to the database.
func (bsm *BackgroundSyncManager) extractAndSaveMetadata(filePath string, imageFileID uint) {
	meta, err := extractMetadata(filePath)
	if err != nil {
		log.Printf("Background sync: failed to extract metadata for %s: %v", filePath, err)
		return
	}

	meta.ImageFileID = imageFileID

	// Enrich with geo data from geocoder if GPS coordinates are present
	if bsm.geocoder != nil && meta.GPSLatitude != nil && meta.GPSLongitude != nil {
		country, city := bsm.geocoder.ReverseGeocode(*meta.GPSLatitude, *meta.GPSLongitude)
		meta.GeoCountry = country
		meta.GeoCity = city
	}

	// Upsert: insert or update
	if err := bsm.db.Where("image_file_id = ?", imageFileID).Assign(meta).FirstOrCreate(&domain.ImageMetadata{}).Error; err != nil {
		log.Printf("Background sync: failed to save metadata for %s: %v", filePath, err)
	} else {
		log.Printf("Background sync: saved EXIF/geo metadata for %s", filePath)
	}
}

// invalidateOCRClassification deletes existing OCR classification for a file
// so it gets re-classified on the next OCR pass.
func (bsm *BackgroundSyncManager) invalidateOCRClassification(imageFileID uint) {
	// Delete bounding boxes first (foreign key dependency)
	bsm.db.Where("classification_id IN (SELECT id FROM ocr_classifications WHERE image_file_id = ?)", imageFileID).Delete(&domain.OcrBoundingBox{})
	// Delete the classification
	if result := bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.OcrClassification{}); result.Error == nil && result.RowsAffected > 0 {
		log.Printf("Background sync: invalidated OCR classification for image %d", imageFileID)
	}
}

// SyncStatus returns the current status of background sync
type SyncStatus struct {
	Running  bool   `json:"running"`
	Interval string `json:"interval"`
}

// GetStatus returns the current sync status
func (bsm *BackgroundSyncManager) GetStatus() SyncStatus {
	return SyncStatus{
		Running:  bsm.IsRunning(),
		Interval: bsm.syncInterval.String(),
	}
}
