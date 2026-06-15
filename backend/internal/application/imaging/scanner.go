package imaging

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"image-toolkit/internal/domain"

	"gorm.io/gorm"
)

// calculateFileHash calculates MD5 hash of a file
func calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fileInfo holds file information collected during directory walk
type fileInfo struct {
	path           string
	normalizedPath string
	size           int64
	modTime        time.Time
}

// hashResult holds the result of a file hash computation
type hashResult struct {
	fi       fileInfo
	hash     string
	err      error
	existing *domain.ImageFile
}

// batchTracker collects file events during DB batch writes and emits them via callback.
type batchTracker struct {
	onFileProcessed func(FileEvent)
	sem             chan struct{}
}

// flush writes accumulated create/update records to the database, resets the slices,
// and emits FileCreated/FileModified events for each affected record.
func (bt *batchTracker) flush(db *gorm.DB, toCreate *[]domain.ImageFile, toUpdate *[]domain.ImageFile) {
	var events []FileEvent

	if len(*toCreate) > 0 {
		db.Create(toCreate)
		for _, f := range *toCreate {
			events = append(events, FileEvent{
				Type:        FileCreated,
				ImageFileID: f.ID,
				Path:        f.Path,
			})
		}
		*toCreate = (*toCreate)[:0]
	}
	for _, f := range *toUpdate {
		contentChanged := true // safe default: treat as content-changed
		if f.Hash != "" {
			var oldHash string
			db.Model(&domain.ImageFile{}).Select("hash").Where("id = ?", f.ID).Scan(&oldHash)
			contentChanged = oldHash != f.Hash
		}
		db.Save(&f)
		events = append(events, FileEvent{
			Type:           FileModified,
			ImageFileID:    f.ID,
			Path:           f.Path,
			ContentChanged: contentChanged,
		})
	}
	*toUpdate = (*toUpdate)[:0]

	// Emit events asynchronously via callback
	if bt.onFileProcessed != nil {
		for _, event := range events {
			if bt.sem != nil {
				bt.sem <- struct{}{}
			}
			e := event
			go func() {
				defer func() { <-bt.sem }()
				bt.onFileProcessed(e)
			}()
		}
	}
}

// scanDirectory scans a directory for image files and updates the database.
// numWorkers controls the number of parallel goroutines used for file hashing.
func scanDirectory(db *gorm.DB, dirPath string, progressChan chan<- string, numWorkers int, onFileProcessed func(FileEvent), postProcessSem chan struct{}) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if numWorkers <= 0 {
		numWorkers = 1
	}

	// Phase 1: Collect all image files from the directory tree
	var allFiles []fileInfo
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			progressChan <- "Error accessing " + path + ": " + err.Error()
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !domain.IsImageFile(path) {
			return nil
		}
		allFiles = append(allFiles, fileInfo{
			path:           path,
			normalizedPath: filepath.ToSlash(path),
			size:           info.Size(),
			modTime:        info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return err
	}

	if len(allFiles) == 0 {
		return nil
	}

	// Phase 2: Batch query existing files from DB to build a cache map
	existingMap := make(map[string]domain.ImageFile)
	const dbBatchSize = 500
	for i := 0; i < len(allFiles); i += dbBatchSize {
		end := i + dbBatchSize
		if end > len(allFiles) {
			end = len(allFiles)
		}
		paths := make([]string, end-i)
		for j, fi := range allFiles[i:end] {
			paths[j] = fi.normalizedPath
		}
		var existingFiles []domain.ImageFile
		db.Where("path IN ?", paths).Find(&existingFiles)
		for _, ef := range existingFiles {
			existingMap[ef.Path] = ef
		}
	}

	// Phase 3: Separate cached (unchanged) files from files that need hashing
	var filesToHash []fileInfo
	for _, fi := range allFiles {
		if existing, ok := existingMap[fi.normalizedPath]; ok {
			if existing.ModTime.Equal(fi.modTime) && existing.Size == fi.size {
				progressChan <- "Skipping (cached): " + fi.path
				continue
			}
		}
		filesToHash = append(filesToHash, fi)
	}

	if len(filesToHash) == 0 {
		return nil
	}

	// Phase 4: Hash files in parallel using a worker pool
	jobs := make(chan fileInfo, numWorkers*2)
	results := make(chan hashResult, numWorkers*2)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range jobs {
				hash, err := calculateFileHash(fi.path)
				var existing *domain.ImageFile
				if ef, ok := existingMap[fi.normalizedPath]; ok {
					existing = &ef
				}
				results <- hashResult{
					fi:       fi,
					hash:     hash,
					err:      err,
					existing: existing,
				}
			}
		}()
	}

	// Close results channel when all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Send jobs to workers
	go func() {
		for _, fi := range filesToHash {
			jobs <- fi
		}
		close(jobs)
	}()

	// Phase 5: Collect results and batch write to DB
	const writeBatchSize = 50
	var toCreate []domain.ImageFile
	var toUpdate []domain.ImageFile
	tracker := &batchTracker{onFileProcessed: onFileProcessed, sem: postProcessSem}

	for result := range results {
		if result.err != nil {
			progressChan <- "Error hashing " + result.fi.path + ": " + result.err.Error()
			continue
		}

		progressChan <- "Processed: " + result.fi.path

		imageFile := domain.ImageFile{
			Path:    result.fi.normalizedPath,
			Size:    result.fi.size,
			Hash:    result.hash,
			ModTime: result.fi.modTime,
		}

		if result.existing != nil {
			imageFile.ID = result.existing.ID
			toUpdate = append(toUpdate, imageFile)
		} else {
			toCreate = append(toCreate, imageFile)
		}

		if len(toCreate)+len(toUpdate) >= writeBatchSize {
			tracker.flush(db, &toCreate, &toUpdate)
		}
	}

	// Flush remaining
	tracker.flush(db, &toCreate, &toUpdate)

	return nil
}

// flushDBBatch writes accumulated create/update records to the database and resets the slices.
// Deprecated: use batchTracker.flush for event-emitting scans.
func flushDBBatch(db *gorm.DB, toCreate *[]domain.ImageFile, toUpdate *[]domain.ImageFile) {
	if len(*toCreate) > 0 {
		db.Create(toCreate)
		*toCreate = (*toCreate)[:0]
	}
	for _, f := range *toUpdate {
		db.Save(&f)
	}
	*toUpdate = (*toUpdate)[:0]
}

// fastScanGalleryDirectory performs a fast gallery scan that only computes hash
// when file record doesn't exist in DB or size differs.
// It also cleans up records for files that no longer exist on disk.
// Returns statistics about the scan operation.
// numWorkers controls the number of parallel goroutines used for file hashing.
func fastScanGalleryDirectory(db *gorm.DB, dirPath string, progressChan chan<- string, numWorkers int, onFileProcessed func(FileEvent), postProcessSem chan struct{}) FastScanResult {
	stats := FastScanResult{}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		progressChan <- "Error: failed to get absolute path: " + err.Error()
		return stats
	}

	if numWorkers <= 0 {
		numWorkers = 1
	}

	// Phase 1: Collect all image files from the directory tree
	var allFiles []fileInfo
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			progressChan <- "Error accessing " + path + ": " + err.Error()
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !domain.IsImageFile(path) {
			return nil
		}
		allFiles = append(allFiles, fileInfo{
			path:           path,
			normalizedPath: filepath.ToSlash(path),
			size:           info.Size(),
			modTime:        info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return stats
	}

	if len(allFiles) == 0 {
		return stats
	}

	// Phase 2: Batch query existing files from DB by path to build a cache map
	// Also track all DB record IDs for later cleanup
	existingMap := make(map[string]domain.ImageFile)
	checkedIDs := make(map[uint]bool) // IDs of files that were checked
	const dbBatchSize = 500
	for i := 0; i < len(allFiles); i += dbBatchSize {
		end := i + dbBatchSize
		if end > len(allFiles) {
			end = len(allFiles)
		}
		paths := make([]string, end-i)
		for j, fi := range allFiles[i:end] {
			paths[j] = fi.normalizedPath
		}
		var existingFiles []domain.ImageFile
		db.Where("path IN ?", paths).Find(&existingFiles)
		for _, ef := range existingFiles {
			existingMap[ef.Path] = ef
			checkedIDs[ef.ID] = true // Mark this ID as checked (exists on disk)
		}
	}

	// Phase 3: Check files - if record exists with matching size, skip hashing
	// Otherwise, compute hash and update/create record
	var filesToProcess []fileInfo
	for _, fi := range allFiles {
		if existing, ok := existingMap[fi.normalizedPath]; ok {
			if existing.Size == fi.size {
				// File exists and size matches - no change needed
				stats.Unchanged++
				progressChan <- "Skipped (unchanged): " + fi.path
				continue
			}
			// Size differs - need to update
			filesToProcess = append(filesToProcess, fi)
			stats.TotalChecked++ // Count modified as checked
		} else {
			// New file - need to create
			filesToProcess = append(filesToProcess, fi)
			stats.TotalChecked++ // Count created as checked
		}
	}

	// Phase 3.5: Delete records for files that don't exist on disk anymore
	// Get all IDs in this directory that were NOT checked
	var existingFilesInDir []domain.ImageFile
	prefix := filepath.ToSlash(absPath) + "/"
	db.Where("path LIKE ?", prefix+"%").Find(&existingFilesInDir)

	for _, ef := range existingFilesInDir {
		if !checkedIDs[ef.ID] {
			// This file exists in DB but not on disk - cascade delete it and all children
			progressChan <- "Removing missing file from DB: " + ef.Path
			deleteImageFileCascade(db, ef.ID)
			stats.Deleted++
			// Emit FileDeleted event for post-processing
			if onFileProcessed != nil {
				event := FileEvent{Type: FileDeleted, ImageFileID: ef.ID, Path: ef.Path}
				if postProcessSem != nil {
					postProcessSem <- struct{}{}
				}
				go func() {
					if postProcessSem != nil {
						defer func() { <-postProcessSem }()
					}
					onFileProcessed(event)
				}()
			}
		}
	}

	if len(filesToProcess) == 0 {
		return stats
	}

	// Phase 4: Hash files in parallel using a worker pool
	jobs := make(chan fileInfo, numWorkers*2)
	results := make(chan hashResult, numWorkers*2)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range jobs {
				hash, err := calculateFileHash(fi.path)
				var existing *domain.ImageFile
				if ef, ok := existingMap[fi.normalizedPath]; ok {
					existing = &ef
				}
				results <- hashResult{
					fi:       fi,
					hash:     hash,
					err:      err,
					existing: existing,
				}
			}
		}()
	}

	// Close results channel when all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Send jobs to workers
	go func() {
		for _, fi := range filesToProcess {
			jobs <- fi
		}
		close(jobs)
	}()

	// Phase 5: Collect results and batch write to DB
	const writeBatchSize = 50
	var toCreate []domain.ImageFile
	var toUpdate []domain.ImageFile
	tracker := &batchTracker{onFileProcessed: onFileProcessed, sem: postProcessSem}

	for result := range results {
		if result.err != nil {
			progressChan <- "Error hashing " + result.fi.path + ": " + result.err.Error()
			continue
		}

		progressChan <- "Processed: " + result.fi.path

		imageFile := domain.ImageFile{
			Path:    result.fi.normalizedPath,
			Size:    result.fi.size,
			Hash:    result.hash,
			ModTime: result.fi.modTime,
		}

		if result.existing != nil {
			imageFile.ID = result.existing.ID
			toUpdate = append(toUpdate, imageFile)
			stats.Modified++
		} else {
			toCreate = append(toCreate, imageFile)
			stats.Created++
		}

		if len(toCreate)+len(toUpdate) >= writeBatchSize {
			tracker.flush(db, &toCreate, &toUpdate)
		}
	}

	// Flush remaining
	tracker.flush(db, &toCreate, &toUpdate)

	// Update total checked count (modified + created)
	stats.TotalChecked = stats.Modified + stats.Created

	return stats
}

// findDuplicates finds all duplicate groups from the database
func findDuplicates(db *gorm.DB) ([]domain.DuplicateGroup, error) {
	type HashSizeCount struct {
		Hash  string
		Size  int64
		Count int64
	}

	var duplicateHashSizes []HashSizeCount
	result := db.Model(&domain.ImageFile{}).
		Select("hash, size, count(*) as count").
		Group("hash, size").
		Having("count(*) > 1").
		Scan(&duplicateHashSizes)

	if result.Error != nil {
		return nil, result.Error
	}

	var groups []domain.DuplicateGroup
	for _, hs := range duplicateHashSizes {
		var files []domain.ImageFile
		db.Where("hash = ? AND size = ?", hs.Hash, hs.Size).Find(&files)

		var existingFiles []domain.ImageFile
		for _, f := range files {
			if _, err := os.Stat(f.Path); err == nil {
				existingFiles = append(existingFiles, f)
			} else {
				db.Delete(&f)
			}
		}

		if len(existingFiles) > 1 {
			groups = append(groups, domain.DuplicateGroup{
				Hash:  hs.Hash,
				Size:  hs.Size,
				Files: existingFiles,
			})
		}
	}

	return groups, nil
}

// FindDuplicatesPaginated finds duplicate groups with pagination
func FindDuplicatesPaginated(db *gorm.DB, offset, limit int) ([]domain.DuplicateGroup, int, int, error) {
	type HashSizeCount struct {
		Hash  string
		Size  int64
		Count int64
	}

	var allDuplicateHashSizes []HashSizeCount
	result := db.Model(&domain.ImageFile{}).
		Select("hash, size, count(*) as count").
		Group("hash, size").
		Having("count(*) > 1").
		Order("size DESC").
		Scan(&allDuplicateHashSizes)

	if result.Error != nil {
		return nil, 0, 0, result.Error
	}

	totalGroups := len(allDuplicateHashSizes)

	totalFiles := 0
	for _, hs := range allDuplicateHashSizes {
		totalFiles += int(hs.Count)
	}

	if offset >= len(allDuplicateHashSizes) {
		return []domain.DuplicateGroup{}, totalGroups, totalFiles, nil
	}

	end := offset + limit
	if end > len(allDuplicateHashSizes) {
		end = len(allDuplicateHashSizes)
	}

	paginatedHashSizes := allDuplicateHashSizes[offset:end]

	var groups []domain.DuplicateGroup
	for _, hs := range paginatedHashSizes {
		var files []domain.ImageFile
		db.Where("hash = ? AND size = ?", hs.Hash, hs.Size).Find(&files)

		if len(files) > 1 {
			groups = append(groups, domain.DuplicateGroup{
				Hash:  hs.Hash,
				Size:  hs.Size,
				Files: files,
			})
		}
	}

	return groups, totalGroups, totalFiles, nil
}

// deleteImageFileCascade deletes an ImageFile and all its dependent records.
// Must be used instead of a bare db.Delete(&imageFile) to prevent orphaned child rows.
func deleteImageFileCascade(db *gorm.DB, imageFileID uint) {
	db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageTag{})
	db.Where("image_file_id = ?", imageFileID).Delete(&domain.OcrLlmRecognition{})
	// Delete bounding boxes before their parent classifications
	db.Where("classification_id IN (SELECT id FROM ocr_classifications WHERE image_file_id = ?)", imageFileID).
		Delete(&domain.OcrBoundingBox{})
	db.Where("image_file_id = ?", imageFileID).Delete(&domain.OcrClassification{})
	db.Where("image_file_id = ?", imageFileID).Delete(&domain.TagEmbedding{})
	db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageMetadata{})
	db.Where("id = ?", imageFileID).Delete(&domain.ImageFile{})
}

// cleanupMissingFiles removes database entries for files that no longer exist
func cleanupMissingFiles(db *gorm.DB, progressChan chan<- string, onFileProcessed func(FileEvent), postProcessSem chan struct{}) error {
	var files []domain.ImageFile
	db.Find(&files)

	for _, f := range files {
		if _, err := os.Stat(f.Path); os.IsNotExist(err) {
			progressChan <- fmt.Sprintf("Removing missing file from DB: %s", f.Path)
			deleteImageFileCascade(db, f.ID)
			// Emit FileDeleted event for post-processing
			if onFileProcessed != nil {
				event := FileEvent{Type: FileDeleted, ImageFileID: f.ID, Path: f.Path}
				if postProcessSem != nil {
					postProcessSem <- struct{}{}
				}
				go func() {
					if postProcessSem != nil {
						defer func() { <-postProcessSem }()
					}
					onFileProcessed(event)
				}()
			}
		}
	}

	return nil
}
