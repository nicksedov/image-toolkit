package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// progressBuffer accumulates progress messages for batch output
type progressBuffer struct {
	messages []string
	limit    int
	channel  chan<- string
}

func newProgressBuffer(ch chan<- string, limit int) *progressBuffer {
	return &progressBuffer{
		messages: make([]string, 0, limit),
		limit:    limit,
		channel:  ch,
	}
}

func (pb *progressBuffer) add(msg string) {
	pb.messages = append(pb.messages, msg)
	if len(pb.messages) >= pb.limit {
		pb.flush()
	}
}

func (pb *progressBuffer) flush() {
	if len(pb.messages) == 0 {
		return
	}
	var sb strings.Builder
	for i, msg := range pb.messages {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(msg)
	}
	pb.channel <- sb.String()
	pb.messages = pb.messages[:0]
}

// scanDirectory scans a directory for image files and updates the database
func scanDirectory(db *gorm.DB, dirPath string, progressChan chan<- string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	const batchSize = 50
	const progressBufferSize = 100
	var batch []fileInfo
	progress := newProgressBuffer(progressChan, progressBufferSize)

	processBatch := func(batch []fileInfo) {
		if len(batch) == 0 {
			return
		}

		paths := make([]string, len(batch))
		pathToInfo := make(map[string]fileInfo)
		for i, fi := range batch {
			paths[i] = fi.normalizedPath
			pathToInfo[fi.normalizedPath] = fi
		}

		var existingFiles []ImageFile
		db.Where("path IN ?", paths).Find(&existingFiles)

		existingMap := make(map[string]ImageFile)
		for _, ef := range existingFiles {
			existingMap[ef.Path] = ef
		}

		var toCreate []ImageFile
		var toUpdate []ImageFile

		for _, fi := range batch {
			existing, exists := existingMap[fi.normalizedPath]

			if exists {
				if existing.ModTime.Equal(fi.modTime) && existing.Size == fi.size {
					progress.add("Skipping (cached): " + fi.path)
					continue
				}
			}

			progress.add("Processing: " + fi.path)

			hash, err := calculateFileHash(fi.path)
			if err != nil {
				progress.add("Error hashing " + fi.path + ": " + err.Error())
				continue
			}

			imageFile := ImageFile{
				Path:    fi.normalizedPath,
				Size:    fi.size,
				Hash:    hash,
				ModTime: fi.modTime,
			}

			if exists {
				imageFile.ID = existing.ID
				toUpdate = append(toUpdate, imageFile)
			} else {
				toCreate = append(toCreate, imageFile)
			}
		}

		if len(toCreate) > 0 {
			db.Create(&toCreate)
		}

		for _, f := range toUpdate {
			db.Save(&f)
		}

		progress.flush()
	}

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			progress.add("Error accessing " + path + ": " + err.Error())
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if !isImageFile(path) {
			return nil
		}

		normalizedPath := filepath.ToSlash(path)

		batch = append(batch, fileInfo{
			path:           path,
			normalizedPath: normalizedPath,
			size:           info.Size(),
			modTime:        info.ModTime(),
		})

		if len(batch) >= batchSize {
			processBatch(batch)
			batch = batch[:0]
		}

		return nil
	})

	if len(batch) > 0 {
		processBatch(batch)
	}

	progress.flush()

	return err
}

// findDuplicates finds all duplicate groups from the database
func findDuplicates(db *gorm.DB) ([]DuplicateGroup, error) {
	type HashSizeCount struct {
		Hash  string
		Size  int64
		Count int64
	}

	var duplicateHashSizes []HashSizeCount
	result := db.Model(&ImageFile{}).
		Select("hash, size, count(*) as count").
		Group("hash, size").
		Having("count(*) > 1").
		Scan(&duplicateHashSizes)

	if result.Error != nil {
		return nil, result.Error
	}

	var groups []DuplicateGroup
	for _, hs := range duplicateHashSizes {
		var files []ImageFile
		db.Where("hash = ? AND size = ?", hs.Hash, hs.Size).Find(&files)

		var existingFiles []ImageFile
		for _, f := range files {
			if _, err := os.Stat(f.Path); err == nil {
				existingFiles = append(existingFiles, f)
			} else {
				db.Delete(&f)
			}
		}

		if len(existingFiles) > 1 {
			groups = append(groups, DuplicateGroup{
				Hash:  hs.Hash,
				Size:  hs.Size,
				Files: existingFiles,
			})
		}
	}

	return groups, nil
}

// findDuplicatesPaginated finds duplicate groups with pagination
func findDuplicatesPaginated(db *gorm.DB, offset, limit int) ([]DuplicateGroup, int, int, error) {
	type HashSizeCount struct {
		Hash  string
		Size  int64
		Count int64
	}

	var allDuplicateHashSizes []HashSizeCount
	result := db.Model(&ImageFile{}).
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
		return []DuplicateGroup{}, totalGroups, totalFiles, nil
	}

	end := offset + limit
	if end > len(allDuplicateHashSizes) {
		end = len(allDuplicateHashSizes)
	}

	paginatedHashSizes := allDuplicateHashSizes[offset:end]

	var groups []DuplicateGroup
	for _, hs := range paginatedHashSizes {
		var files []ImageFile
		db.Where("hash = ? AND size = ?", hs.Hash, hs.Size).Find(&files)

		if len(files) > 1 {
			groups = append(groups, DuplicateGroup{
				Hash:  hs.Hash,
				Size:  hs.Size,
				Files: files,
			})
		}
	}

	return groups, totalGroups, totalFiles, nil
}

// cleanupMissingFiles removes database entries for files that no longer exist
func cleanupMissingFiles(db *gorm.DB, progressChan chan<- string) error {
	var files []ImageFile
	db.Find(&files)

	for _, f := range files {
		if _, err := os.Stat(f.Path); os.IsNotExist(err) {
			progressChan <- fmt.Sprintf("Removing missing file from DB: %s", f.Path)
			db.Delete(&f)
		}
	}

	return nil
}
