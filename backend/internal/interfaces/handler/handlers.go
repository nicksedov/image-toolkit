package handler

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"image-toolkit/internal/application/geo"
	"image-toolkit/internal/application/imaging"
	"image-toolkit/internal/application/thumbnail"
	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"
	"image-toolkit/internal/interfaces/dto"
	"image-toolkit/internal/interfaces/handler/helpers"
	"image-toolkit/internal/interfaces/i18n"
	"image-toolkit/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// handleGetDuplicates returns paginated duplicate groups as JSON
func (s *Server) handleGetDuplicates(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFixed)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset

	groups, totalGroups, totalFiles, err := imaging.FindDuplicatesPaginated(s.db, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	pag := helpers.CalcPagination(page, pageSize, int64(totalGroups))

	// Prepare group DTOs with parallel thumbnail generation
	groupDTOs := make([]dto.DuplicateGroupDTO, len(groups))
	pageFiles := 0

	for _, g := range groups {
		pageFiles += len(g.Files)
	}

	// Collect paths for thumbnail generation
	paths := make([]string, len(groups))
	pathToIdx := make(map[string]int)
	for i, g := range groups {
		fileDTOs := make([]dto.FileDTO, len(g.Files))
		for j, f := range g.Files {
			fileDTOs[j] = dto.FileDTO{
				ID:       f.ID,
				Path:     f.Path,
				FileName: filepath.Base(f.Path),
				DirPath:  filepath.Dir(f.Path),
				ModTime:  f.ModTime.Format(helpers.DateTimeFormat),
			}
		}

		groupDTOs[i] = dto.DuplicateGroupDTO{
			Index:     offset + i + 1,
			Hash:      g.Hash,
			Size:      g.Size,
			SizeHuman: formatSize(g.Size),
			Files:     fileDTOs,
		}

		if len(g.Files) > 0 {
			paths[i] = g.Files[0].Path
			pathToIdx[g.Files[0].Path] = i
		}
	}

	s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
		groupDTOs[idx].Thumbnail = thumb
	})

	// Get scanned dirs from gallery folders
	var galleryFolders []domain.GalleryFolder
	s.db.Order("created_at").Find(&galleryFolders)
	scannedDirs := make([]string, len(galleryFolders))
	for i, f := range galleryFolders {
		scannedDirs[i] = f.Path
	}

	response := dto.DuplicatesResponse{
		Groups:      groupDTOs,
		TotalFiles:  totalFiles,
		PageFiles:   pageFiles,
		TotalGroups: totalGroups,
		ScannedDirs: scannedDirs,
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasPrevPage: pag.HasPrevPage,
		HasNextPage: pag.HasNextPage,
		PageSizes:   helpers.FixedPageSizes,
	}

	c.JSON(http.StatusOK, response)
}

// handleScan triggers an async scan of directories
func (s *Server) handleScan(c *gin.Context) {
	if err := s.scanManager.StartScan(); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgScanFailed))
		return
	}
	c.JSON(http.StatusAccepted, dto.ScanResponse{Message: string(i18n.MsgScanStarted)})
}

// handleFastScan triggers an async fast scan of directories
// Fast scan only computes hash when file record doesn't exist or size differs
func (s *Server) handleFastScan(c *gin.Context) {
	result := s.scanManager.FastScanGallery()
	c.JSON(http.StatusOK, dto.FastScanResponse{
		Message:   string(i18n.MsgScanStarted),
		Unchanged: result.Unchanged,
		Modified:  result.Modified,
		Created:   result.Created,
		Deleted:   result.Deleted,
		Total:     result.TotalChecked,
	})
}

// handleGetStatus returns the current scan status
func (s *Server) handleGetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.scanManager.GetStatus())
}

// handleThumbnail serves a thumbnail for a specific file
func (s *Server) handleThumbnail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	var thumbnail string
	var err error

	// Use thumbnail service if available
	if s.thumbnailService != nil {
		thumbnail, err = s.thumbnailService.GetOrGenerate(path)
	} else {
		thumbnail, err = imaging.GenerateThumbnail(path, s.thumbnailCache)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	c.JSON(http.StatusOK, dto.ThumbnailResponse{Thumbnail: thumbnail})
}

// handleDeleteFiles deletes selected files directly (moves to trash)
func (s *Server) handleDeleteFiles(c *gin.Context) {
	var req dto.DeleteFilesRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgScanNoFilesSelected))
		return
	}

	result := s.fileMover.BatchProcess(req.FilePaths, req.TrashDir)

	c.JSON(http.StatusOK, dto.DeleteFilesResponse{
		Success:     result.Success,
		Failed:      result.Failed,
		FailedFiles: result.FailedFiles,
	})
}

// handleGetFolderPatterns returns all unique folder patterns from duplicates
func (s *Server) handleGetFolderPatterns(c *gin.Context) {
	groups, _, _, err := imaging.FindDuplicatesPaginated(s.db, 0, 100000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	patternMap := make(map[string]*dto.FolderPattern)
	singleFolderDuplicateCount := 0

	for _, group := range groups {
		folderSet := make(map[string]bool)
		for _, file := range group.Files {
			dir := filepath.Dir(file.Path)
			folderSet[dir] = true
		}

		// Skip groups where all duplicates are in a single folder
		// These can't be handled by batch dedup (no cross-folder choice to make)
		if len(folderSet) <= 1 {
			singleFolderDuplicateCount += len(group.Files)
			continue
		}

		folders := make([]string, 0, len(folderSet))
		for folder := range folderSet {
			folders = append(folders, folder)
		}

		slices.Sort(folders)

		patternID := createPatternID(folders)

		if existing, ok := patternMap[patternID]; ok {
			existing.DuplicateCount++
			existing.TotalFiles += len(group.Files)
		} else {
			patternMap[patternID] = &dto.FolderPattern{
				ID:             patternID,
				Folders:        folders,
				DuplicateCount: 1,
				TotalFiles:     len(group.Files),
			}
		}
	}

	patterns := make([]dto.FolderPattern, 0, len(patternMap))
	for _, p := range patternMap {
		patterns = append(patterns, *p)
	}

	sortPatternsByCount(patterns)

	c.JSON(http.StatusOK, dto.FolderPatternsResponse{
		Patterns:                   patterns,
		SingleFolderDuplicateCount: singleFolderDuplicateCount,
	})
}

// handleBatchDelete applies batch deletion rules to all matching duplicates
func (s *Server) handleBatchDelete(c *gin.Context) {
	var req dto.BatchDeleteRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.Rules) == 0 {
		c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	ruleMap := make(map[string]string)
	for _, rule := range req.Rules {
		ruleMap[rule.PatternID] = rule.KeepFolder
	}

	groups, _, _, err := imaging.FindDuplicatesPaginated(s.db, 0, 100000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	var rulesApplied, filesDeleted, failedCount int
	var failedFiles []string

	for _, group := range groups {
		folderSet := make(map[string]bool)
		for _, file := range group.Files {
			dir := filepath.Dir(file.Path)
			folderSet[dir] = true
		}

		folders := make([]string, 0, len(folderSet))
		for folder := range folderSet {
			folders = append(folders, folder)
		}
		slices.Sort(folders)

		patternID := createPatternID(folders)

		keepFolder, hasRule := ruleMap[patternID]
		if !hasRule {
			continue
		}

		rulesApplied++

		for _, file := range group.Files {
			fileDir := filepath.Dir(file.Path)
			if fileDir == keepFolder {
				continue
			}

			if err := s.fileMover.MoveToTrashOrDelete(file.Path, req.TrashDir); err != nil {
				failedCount++
				failedFiles = append(failedFiles, filepath.Base(file.Path)+": "+err.Error())
				continue
			}

			s.fileMover.DeleteFromDB(file.Path)
			filesDeleted++
		}
	}

	c.JSON(http.StatusOK, dto.BatchDeleteResponse{
		RulesApplied: rulesApplied,
		FilesDeleted: filesDeleted,
		Failed:       failedCount,
		FailedFiles:  failedFiles,
	})
}

// --- Gallery Folder Handlers ---

// handleGetFolders returns all gallery folders
func (s *Server) handleGetFolders(c *gin.Context) {
	var folders []domain.GalleryFolder
	s.db.Order("created_at").Find(&folders)

	folderDTOs := make([]dto.GalleryFolderDTO, len(folders))
	for i, f := range folders {
		var count int64
		prefix := f.Path + "/"
		s.db.Model(&domain.ImageFile{}).Where("path LIKE ?", prefix+"%").Count(&count)

		folderDTOs[i] = dto.GalleryFolderDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileCount: int(count),
			CreatedAt: f.CreatedAt.Format(helpers.DateTimeFormat),
		}
	}

	c.JSON(http.StatusOK, dto.GalleryFoldersResponse{
		Folders:      folderDTOs,
		TotalFolders: len(folderDTOs),
	})
}

// handleAddFolder adds a new gallery folder and triggers a scan
func (s *Server) handleAddFolder(c *gin.Context) {
	var req dto.AddFolderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Validate directory exists
	normalizedPath, err := helpers.ValidateDirectory(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgFolderInvalidPath))
		return
	}

	// Check conflict with trash directory
	settings := s.settingsLoader.AppSettings()
	if settings.TrashDir != "" {
		if helpers.CheckPathsConflict(normalizedPath, settings.TrashDir) {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgFolderConflictTrash))
			return
		}
	}

	folder := domain.GalleryFolder{Path: normalizedPath}
	if result := s.db.Create(&folder); result.Error != nil {
		if strings.Contains(result.Error.Error(), "duplicate") || strings.Contains(result.Error.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgFolderAlreadyInGallery))
			return
		}
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgFolderAddFailed))
		return
	}

	// Trigger background scan for this folder
	scanStarted := true
	if err := s.scanManager.ScanSingleDir(normalizedPath); err != nil {
		scanStarted = false
	}

	c.JSON(http.StatusOK, dto.AddFolderResponse{
		Message: string(i18n.MsgFolderAdded),
		Folder: dto.GalleryFolderDTO{
			ID:        folder.ID,
			Path:      folder.Path,
			FileCount: 0,
			CreatedAt: folder.CreatedAt.Format(helpers.DateTimeFormat),
		},
		ScanStarted: scanStarted,
	})
}

// handleRemoveFolder removes a gallery folder and its files from the database
func (s *Server) handleRemoveFolder(c *gin.Context) {
	id := c.Param("id")

	var folder domain.GalleryFolder
	if result := s.db.First(&folder, id); result.Error != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgFolderNotFound))
		return
	}

	// Delete all image files under this folder
	prefix := folder.Path + "/"
	result := s.db.Where("path LIKE ?", prefix+"%").Delete(&domain.ImageFile{})
	filesRemoved := int(result.RowsAffected)

	// Delete the folder record
	s.db.Delete(&folder)

	c.JSON(http.StatusOK, dto.RemoveFolderResponse{
		Message:      string(i18n.MsgFolderRemoved),
		FilesRemoved: filesRemoved,
	})
}

// handleGetGalleryImages returns paginated gallery images
func (s *Server) handleGetGalleryImages(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFlexible)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset
	view := c.DefaultQuery("view", "list")
	sortOrder := c.DefaultQuery("sortOrder", "newest")
	searchQuery := c.DefaultQuery("search", "")

	// Build base query with optional search filter
	query := s.db.Model(&domain.ImageFile{})
	if searchQuery != "" {
		// Search in filename or directory path (case-insensitive)
		// PostgreSQL: ILIKE for case-insensitive matching
		pattern := "%" + searchQuery + "%"
		query = query.Where("path ILIKE ?", pattern)
	}

	var totalImages int64
	query.Count(&totalImages)

	pag := helpers.CalcPagination(page, pageSize, totalImages)

	var files []domain.ImageFile
	orderClause := "mod_time DESC"
	if sortOrder == "oldest" {
		orderClause = "mod_time ASC"
	}
	query.Order(orderClause).Offset(offset).Limit(pageSize).Find(&files)

	imageDTOs := make([]dto.GalleryImageDTO, len(files))
	for i, f := range files {
		imageDTOs[i] = dto.GalleryImageDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileName:  filepath.Base(f.Path),
			DirPath:   filepath.Dir(f.Path),
			Size:      f.Size,
			SizeHuman: formatSize(f.Size),
			ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
		}
	}

	// Generate thumbnails in parallel if thumbnail or folders view
	if (view == "thumbnails" || view == "folders") && len(files) > 0 {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
	}

	c.JSON(http.StatusOK, dto.GalleryImagesResponse{
		Images:      imageDTOs,
		TotalImages: int(totalImages),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleServeImage serves a full-size image file
func (s *Server) handleServeImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Security: verify the path is within a gallery folder
	if !s.galleryAccess.VerifyGalleryAccess(c, path) {
		return
	}

	// Convert slash path to OS path for file serving
	osPath := filepath.FromSlash(path)

	if _, err := os.Stat(osPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		return
	}

	c.File(osPath)
}

// handleServeOcrImage serves an image scaled and rotated for OCR overlay display
func (s *Server) handleServeOcrImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	angleStr := c.DefaultQuery("angle", "0")
	scaleFactorStr := c.DefaultQuery("scaleFactor", "1")

	angle, err := strconv.ParseFloat(angleStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	scaleFactor, err := strconv.ParseFloat(scaleFactorStr, 64)
	if err != nil || scaleFactor <= 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Security: verify the path is within a gallery folder
	if !s.galleryAccess.VerifyGalleryAccess(c, path) {
		return
	}

	osPath := filepath.FromSlash(path)

	data, err := imaging.PrepareOcrImage(osPath, scaleFactor, angle)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
		} else {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		}
		return
	}

	c.Data(http.StatusOK, "image/webp", data)
}

// --- App Settings Handlers ---

// handleGetSettings returns the current application settings
func (s *Server) handleGetSettings(c *gin.Context) {
	settings := s.settingsLoader.AppSettings()
	c.JSON(http.StatusOK, dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		DailySyncEnabled:      settings.DailySyncEnabled,
		DailySyncHour:         settings.DailySyncHour,
		DailySyncMinute:       settings.DailySyncMinute,
	})
}

// handleUpdateSettings updates the application settings
func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req dto.UpdateSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	settings := s.settingsLoader.AppSettings()

	if req.TrashDir != nil {
		newTrashDir := strings.TrimSpace(*req.TrashDir)
		if newTrashDir != "" {
			// Normalize the trash dir path
			absTrash, err := filepath.Abs(newTrashDir)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTrashPath))
				return
			}
			normalizedTrash := filepath.ToSlash(absTrash)

			// Check conflict with all gallery folders
			var galleryFolders []domain.GalleryFolder
			s.db.Find(&galleryFolders)
			for _, gf := range galleryFolders {
				if helpers.CheckPathsConflict(normalizedTrash, gf.Path) {
					c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageTrashConflict))
					return
				}
			}
			settings.TrashDir = normalizedTrash
		} else {
			settings.TrashDir = ""
		}
	}
	if req.ThumbnailCachePath != nil {
		newCachePath := strings.TrimSpace(*req.ThumbnailCachePath)
		if newCachePath != "" {
			// Normalize the cache path
			absCache, err := filepath.Abs(newCachePath)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTrashPath))
				return
			}
			normalizedCache := filepath.ToSlash(absCache)
			settings.ThumbnailCachePath = normalizedCache

			// Update thumbnail service if available
			if s.thumbnailService != nil {
				log.Printf("Updating thumbnail cache path from %s to %s", s.thumbnailService.Stats().CacheDir, normalizedCache)
				if err := s.thumbnailService.UpdateCachePath(normalizedCache); err != nil {
					log.Printf("Failed to update thumbnail cache path: %v", err)
				} else {
					log.Printf("Thumbnail cache path updated successfully. New stats: %+v", s.thumbnailService.Stats())
				}
			} else {
				log.Printf("Thumbnail service is nil, cannot update cache path")
			}
		} else {
			settings.ThumbnailCachePath = ""
		}
	}

	if req.OcrConcurrentRequests != nil {
		val := *req.OcrConcurrentRequests
		if val < 0 {
			val = 0
		}
		settings.OcrConcurrentRequests = val

		// Update OcrManager in real-time if it exists
		if s.ocrManager != nil {
			s.ocrManager.SetMaxWorkers(val)
		}
	}

	if req.DailySyncEnabled != nil {
		settings.DailySyncEnabled = *req.DailySyncEnabled
	}
	if req.DailySyncHour != nil {
		hour := *req.DailySyncHour
		if hour < 0 || hour > 23 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		settings.DailySyncHour = hour
	}
	if req.DailySyncMinute != nil {
		minute := *req.DailySyncMinute
		if minute < 0 || minute > 59 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		settings.DailySyncMinute = minute
	}

	// Apply schedule changes to background sync manager in real-time
	if s.backgroundSync != nil && (req.DailySyncEnabled != nil || req.DailySyncHour != nil || req.DailySyncMinute != nil) {
		s.backgroundSync.UpdateSchedule(settings.DailySyncEnabled, settings.DailySyncHour, settings.DailySyncMinute)
	}

	s.db.Save(&settings)

	c.JSON(http.StatusOK, dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		DailySyncEnabled:      settings.DailySyncEnabled,
		DailySyncHour:         settings.DailySyncHour,
		DailySyncMinute:       settings.DailySyncMinute,
	})
}

// --- User Settings Handlers ---

// handleGetUserSettings returns the current user's settings
func (s *Server) handleGetUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var settings domain.UserSettings
	if result := s.db.FirstOrCreate(&settings, domain.UserSettings{UserID: user.ID}); result.Error != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}

// handleUpdateUserSettings updates the current user's settings
func (s *Server) handleUpdateUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var req dto.UpdateUserSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	validThemes := map[string]bool{
		"light-purple":  true,
		"dark-purple":   true,
		"light-green":   true,
		"dark-green":    true,
		"light-blue":    true,
		"dark-blue":     true,
		"light-orange":  true,
		"dark-orange":   true,
		"dark-contrast": true,
	}
	validLanguages := map[string]bool{"en": true, "ru": true}

	if req.Theme != "" && !validThemes[req.Theme] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTheme))
		return
	}
	if req.Language != "" && !validLanguages[req.Language] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidLanguage))
		return
	}

	var settings domain.UserSettings
	result := s.db.FirstOrCreate(&settings, domain.UserSettings{UserID: user.ID})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	if req.Theme != "" {
		settings.Theme = req.Theme
	}
	if req.Language != "" {
		settings.Language = req.Language
	}

	s.db.Save(&settings)

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}

// handleGetTrashInfo returns information about files in the trash directory
func (s *Server) handleGetTrashInfo(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusOK, dto.TrashInfoResponse{FileCount: 0, TotalSize: 0, TotalSizeHuman: "0 B"})
		return
	}

	var fileCount int
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileCount++
		if fi, err := entry.Info(); err == nil {
			totalSize += fi.Size()
		}
	}

	c.JSON(http.StatusOK, dto.TrashInfoResponse{
		FileCount:      fileCount,
		TotalSize:      totalSize,
		TotalSizeHuman: formatSize(totalSize),
	})
}

// handleCleanTrash removes all files from the trash directory
func (s *Server) handleCleanTrash(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotExists))
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashReadFailed))
		return
	}

	var deleted, failed int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(settings.TrashDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			failed++
		} else {
			deleted++
		}
	}

	c.JSON(http.StatusOK, dto.CleanTrashResponse{
		Deleted: deleted,
		Failed:  failed,
	})
}

// TrashFileInfo represents a single file in trash
type TrashFileInfo struct {
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"sizeHuman"`
	ModTime   string `json:"modTime"`
}

// handleListTrashFiles returns a list of all files in the trash directory
func (s *Server) handleListTrashFiles(c *gin.Context) {
	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	info, err := os.Stat(settings.TrashDir)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	entries, err := os.ReadDir(settings.TrashDir)
	if err != nil {
		c.JSON(http.StatusOK, []TrashFileInfo{})
		return
	}

	files := make([]TrashFileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if fi, err := entry.Info(); err == nil {
			files = append(files, TrashFileInfo{
				FileName:  entry.Name(),
				Size:      fi.Size(),
				SizeHuman: formatSize(fi.Size()),
				ModTime:   fi.ModTime().Format(helpers.RFC3339Format),
			})
		}
	}

	c.JSON(http.StatusOK, files)
}

// handleRestoreTrashFile moves a file from trash back to the original location
// If targetPath is not provided, restores to current working directory
func (s *Server) handleRestoreTrashFile(c *gin.Context) {
	var req struct {
		FileName   string `json:"fileName"`
		TargetPath string `json:"targetPath"` // Where to restore the file
	}
	if !helpers.BindJSON(c, &req) || req.FileName == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashFileNameRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	restoredPath, err := helpers.RestoreFile(settings.TrashDir, req.FileName, req.TargetPath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashRestored, "restoredPath": restoredPath})
}

// handleDeleteTrashFile permanently deletes a single file from trash
func (s *Server) handleDeleteTrashFile(c *gin.Context) {
	var req struct {
		FileName string `json:"fileName"`
	}
	if !helpers.BindJSON(c, &req) || req.FileName == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashFileNameRequired))
		return
	}

	settings, found := s.settingsLoader.AppSettingsIfExists()
	if !found || settings.TrashDir == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgTrashNotConfigured))
		return
	}

	filePath := filepath.Join(settings.TrashDir, req.FileName)
	if _, err := os.Stat(filePath); err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgTrashFileNotFound))
		return
	}

	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgTrashDeleteFailed))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": i18n.MsgTrashFileDeleted})
}

// handleGetImageMetadata returns EXIF metadata for a single image
func (s *Server) handleGetImageMetadata(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	// Find the image file in DB
	var imageFile domain.ImageFile
	if result := s.db.Where("path = ?", path).First(&imageFile); result.Error != nil {
		c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: false})
		return
	}

	// Find metadata for this image
	var meta domain.ImageMetadata
	if result := s.db.Where("image_file_id = ?", imageFile.ID).First(&meta); result.Error != nil {
		c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: false})
		return
	}

	// Build the DTO
	metaDTO := &dto.ImageMetadataDTO{
		Width:        meta.Width,
		Height:       meta.Height,
		Dimensions:   fmt.Sprintf("%d \u00d7 %d", meta.Width, meta.Height),
		CameraModel:  meta.CameraModel,
		LensModel:    meta.LensModel,
		ISO:          meta.ISO,
		Aperture:     meta.Aperture,
		ShutterSpeed: meta.ShutterSpeed,
		FocalLength:  meta.FocalLength,
		Orientation:  meta.Orientation,
		ColorSpace:   meta.ColorSpace,
		Software:     meta.Software,
		GPSLatitude:  meta.GPSLatitude,
		GPSLongitude: meta.GPSLongitude,
		GeoCountry:   meta.GeoCountry,
		GeoCity:      meta.GeoCity,
		HasGPS:       meta.GPSLatitude != nil && meta.GPSLongitude != nil,
		HasExif:      imaging.HasExifData(&meta),
	}

	if meta.DateTaken != nil {
		metaDTO.DateTaken = meta.DateTaken.Format(helpers.DateTimeFormat)
	}

	c.JSON(http.StatusOK, dto.ImageMetadataResponse{Found: true, Metadata: metaDTO})
}

// handleGetGalleryCalendar returns paginated gallery images grouped by date taken
// Supports both cursor-based pagination (new) and offset-based pagination (legacy)
func (s *Server) handleGetGalleryCalendar(c *gin.Context) {
	startDate := c.Query("startDate") // "YYYY-MM-DD" or empty
	endDate := c.Query("endDate")     // "YYYY-MM-DD" or empty
	monthYear := c.Query("monthYear") // "YYYY-MM" for calendar widget
	sortOrder := c.DefaultQuery("sortOrder", "oldest")

	// Query: join image_files with image_metadata where date_taken is not null
	type imageWithDate struct {
		domain.ImageFile
		DateTaken time.Time
	}

	baseQuery := s.db.Table("image_files").
		Select("image_files.*, image_metadata.date_taken").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken IS NOT NULL")

	// Apply date range filter
	if startDate != "" {
		if t, err := time.Parse(helpers.DateOnlyFormat, startDate); err == nil {
			baseQuery = baseQuery.Where("image_metadata.date_taken >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse(helpers.DateOnlyFormat, endDate); err == nil {
			// End of the end date
			endOfDay := t.Add(24*time.Hour - time.Second)
			baseQuery = baseQuery.Where("image_metadata.date_taken <= ?", endOfDay)
		}
	}

	// Count total
	var totalImages int64
	baseQuery.Count(&totalImages)

	// Determine pagination mode: cursor or page-based
	cursorParam := c.Query("cursor")
	var results []imageWithDate
	var nextCursor *string

	if cursorParam != "" {
		// Cursor-based pagination - recreate baseQuery from scratch to avoid mutation issues
		decodedDate, decodedID, err := helpers.DecodeCursor(cursorParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidCursor))
			return
		}

		// Parse the decoded date
		cursorDate, err := time.Parse(helpers.DateOnlyFormat, decodedDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidCursor))
			return
		}

		pageSize := 50
		if ps := c.Query("pageSize"); ps != "" {
			if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 200 {
				pageSize = parsed
			}
		}

		// Build order clause
		orderClause := "image_metadata.date_taken ASC, image_files.id ASC"
		if sortOrder == "newest" {
			orderClause = "image_metadata.date_taken DESC, image_files.id DESC"
		}

		// Create a FRESH baseQuery for cursor pagination to avoid state mutation
		cursorQuery := s.db.Table("image_files").
			Select("image_files.*, image_metadata.date_taken").
			Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
			Where("image_metadata.date_taken IS NOT NULL")

		// Apply date range filters
		if startDate != "" {
			if t, err := time.Parse(helpers.DateOnlyFormat, startDate); err == nil {
				cursorQuery = cursorQuery.Where("image_metadata.date_taken >= ?", t)
			}
		}
		if endDate != "" {
			if t, err := time.Parse(helpers.DateOnlyFormat, endDate); err == nil {
				endOfDay := t.Add(24*time.Hour - time.Second)
				cursorQuery = cursorQuery.Where("image_metadata.date_taken <= ?", endOfDay)
			}
		}

		// Query with cursor: fetch limit+1 to detect if more exists
		query := cursorQuery.Order(orderClause).Limit(pageSize + 1)

		if sortOrder == "newest" {
			// For newest first: get items before the cursor
			query = query.Where(
				"(image_metadata.date_taken < ?) OR (image_metadata.date_taken = ? AND image_files.id < ?)",
				cursorDate, cursorDate, decodedID,
			)
		} else {
			// For oldest first: get items after the cursor
			query = query.Where(
				"(image_metadata.date_taken > ?) OR (image_metadata.date_taken = ? AND image_files.id > ?)",
				cursorDate, cursorDate, decodedID,
			)
		}
		
		// Debug: show query parameters
		fmt.Printf("[Calendar Debug] Query params: date='%s', id=%d, sortOrder=%s\n", 
			cursorDate.Format("2006-01-02"), decodedID, sortOrder)

		query.Find(&results)

		// Debug logging for cursor pagination
		fmt.Printf("[Calendar Debug] Cursor: %s, Decoded: date=%s, id=%d\n", cursorParam, decodedDate, decodedID)
		fmt.Printf("[Calendar Debug] Results count: %d\n", len(results))
		if len(results) > 0 {
			lastResult := results[len(results)-1]
			fmt.Printf("[Calendar Debug] Last result: date=%v, id=%d\n", lastResult.DateTaken, lastResult.ID)
		}

		// If we got more than pageSize, the last item is used for next cursor
		if len(results) > pageSize {
			lastItem := results[pageSize]
			cursorStr := helpers.EncodeCursor(lastItem.DateTaken.Format(helpers.DateOnlyFormat), lastItem.ID)
			nextCursor = &cursorStr
			results = results[:pageSize] // Drop the extra item

			fmt.Printf("[Calendar Debug] Generated nextCursor: %s (from item at index %d, id=%d)\n", *nextCursor, pageSize, lastItem.ID)
		}
	} else {
		// Legacy offset-based pagination (first page or when no cursor provided)
		params := helpers.ParsePagination(c, helpers.ModeFlexible)
		pageSize := params.PageSize
		offset := params.Offset

		orderClause := "image_metadata.date_taken ASC"
		if sortOrder == "newest" {
			orderClause = "image_metadata.date_taken DESC"
		}

		// Fetch pageSize + 1 to detect if there's more data and generate a cursor
		baseQuery.Order(orderClause).Offset(offset).Limit(pageSize + 1).Find(&results)

		// If we got more than pageSize, generate a next cursor from the last item
		if len(results) > pageSize {
			lastItem := results[pageSize]
			cursorStr := helpers.EncodeCursor(lastItem.DateTaken.Format(helpers.DateOnlyFormat), lastItem.ID)
			nextCursor = &cursorStr
			results = results[:pageSize] // Drop the extra item
		}
	}

	// Group by date
	type dateGroup struct {
		date   time.Time
		images []domain.ImageFile
	}
	groupsMap := make(map[string]*dateGroup)
	var dateOrder []string

	for _, r := range results {
		dateStr := r.DateTaken.Format(helpers.DateOnlyFormat)
		if _, ok := groupsMap[dateStr]; !ok {
			groupsMap[dateStr] = &dateGroup{date: r.DateTaken}
			dateOrder = append(dateOrder, dateStr)
		}
		groupsMap[dateStr].images = append(groupsMap[dateStr].images, r.ImageFile)
	}

	// Build response groups
	groupDTOs := make([]dto.CalendarDateGroup, 0, len(dateOrder))
	for _, dateStr := range dateOrder {
		g := groupsMap[dateStr]
		imageDTOs := make([]dto.GalleryImageDTO, len(g.images))
		for i, f := range g.images {
			imageDTOs[i] = dto.GalleryImageDTO{
				ID:        f.ID,
				Path:      f.Path,
				FileName:  filepath.Base(f.Path),
				DirPath:   filepath.Dir(f.Path),
				Size:      f.Size,
				SizeHuman: formatSize(f.Size),
				ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
			}
		}

		// Generate thumbnails in parallel
		if len(g.images) > 0 {
			paths := make([]string, len(g.images))
			for i, f := range g.images {
				paths[i] = f.Path
			}
			s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
				imageDTOs[idx].Thumbnail = thumb
			})
		}

		// Human-readable label
		label := g.date.Format("Monday, January 2, 2006")

		groupDTOs = append(groupDTOs, dto.CalendarDateGroup{
			Date:       dateStr,
			Label:      label,
			ImageCount: len(g.images),
			Images:     imageDTOs,
		})
	}

	// Get date range
	var dateRange dto.CalendarDateRange
	var minDate, maxDate *time.Time
	s.db.Raw("SELECT MIN(date_taken), MAX(date_taken) FROM image_metadata WHERE date_taken IS NOT NULL").Row().Scan(&minDate, &maxDate)
	if minDate != nil {
		dateRange.MinDate = minDate.Format(helpers.DateOnlyFormat)
	}
	if maxDate != nil {
		dateRange.MaxDate = maxDate.Format(helpers.DateOnlyFormat)
	}
	dateRange.TotalWithDate = int(totalImages)

	// Get month info for calendar widget
	var months []dto.CalendarMonthInfo
	if monthYear != "" {
		if t, err := time.Parse(helpers.YearMonthFormat, monthYear); err == nil {
			year := t.Year()
			month := int(t.Month())
			nextMonth := t.AddDate(0, 1, 0)

			// Get distinct days that have images in this month (PostgreSQL)
			var days []int
			s.db.Raw(`
				SELECT DISTINCT CAST(EXTRACT(DAY FROM date_taken) AS INTEGER) as day
				FROM image_metadata
				WHERE date_taken >= $1 AND date_taken < $2 AND date_taken IS NOT NULL
				ORDER BY day
			`, t, nextMonth).Pluck("day", &days)

			months = append(months, dto.CalendarMonthInfo{
				Year:  year,
				Month: month,
				Days:  days,
			})
		}
	}

	// Calculate hasMore for legacy mode or use cursor result
	hasMore := nextCursor != nil
	if cursorParam == "" {
		// Legacy mode: calculate from page
		params := helpers.ParsePagination(c, helpers.ModeFlexible)
		pag := helpers.CalcPagination(params.Page, params.PageSize, totalImages)
		hasMore = pag.HasNextPage
	}

	c.JSON(http.StatusOK, dto.GalleryCalendarResponse{
		Groups:      groupDTOs,
		TotalImages: int(totalImages),
		TotalGroups: len(groupDTOs),
		HasMore:     hasMore,
		DateRange:   dateRange,
		Months:      months,
		NextCursor:  nextCursor,
	})
}

// handleGetCalendarAllDates returns all dates that have images with their counts (lightweight, no thumbnails)
func (s *Server) handleGetCalendarAllDates(c *gin.Context) {
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	// Get min/max dates using the same JOIN as calendar API for consistency
	var minDate, maxDate *time.Time
	s.db.Raw("SELECT MIN(im.date_taken), MAX(im.date_taken) FROM image_files f INNER JOIN image_metadata im ON im.image_file_id = f.id WHERE im.date_taken IS NOT NULL").Row().Scan(&minDate, &maxDate)

	if minDate == nil || maxDate == nil {
		c.JSON(http.StatusOK, dto.CalendarAllDatesResponse{
			MinDate: "",
			MaxDate: "",
			Dates:   []dto.TimelineDateMarker{},
		})
		return
	}

	minDateStr := minDate.Format(helpers.DateOnlyFormat)
	maxDateStr := maxDate.Format(helpers.DateOnlyFormat)

	// Get all dates with image counts, ordered by date ASC (oldest first)
	// Use the same JOIN as the calendar API to ensure consistent counts
	type dateCount struct {
		Date  time.Time
		Count int64
	}
	var dateCounts []dateCount
	s.db.Raw(`
		SELECT DATE(im.date_taken) as date, COUNT(*) as count
		FROM image_files f
		INNER JOIN image_metadata im ON im.image_file_id = f.id
		WHERE im.date_taken IS NOT NULL
		GROUP BY DATE(im.date_taken)
		ORDER BY date ASC
	`).Scan(&dateCounts)

	// Compute page number and cursor for each date.
	// The calendar API paginates by images (not dates), ordered ASC.
	// We track a running total of images to determine which page each date falls on.
	// We also generate a cursor pointing to the start of each date.
	dates := make([]dto.TimelineDateMarker, 0, len(dateCounts))
	imageIndex := 0
	for _, dc := range dateCounts {
		page := (imageIndex / pageSize) + 1
		// Generate a synthetic cursor pointing to the start of this date
		// Using ID 1 as a placeholder - the cursor pagination will find the first image >= this date
		cursor := helpers.EncodeCursor(dc.Date.Format(helpers.DateOnlyFormat), 1)

		dates = append(dates, dto.TimelineDateMarker{
			Date:       dc.Date.Format(helpers.DateOnlyFormat),
			ImageCount: int(dc.Count),
			Page:       page, // Deprecated, kept for backward compatibility
			Cursor:     cursor,
		})
		imageIndex += int(dc.Count)
	}

	c.JSON(http.StatusOK, dto.CalendarAllDatesResponse{
		MinDate: minDateStr,
		MaxDate: maxDateStr,
		Dates:   dates,
	})
}

// handleGetCalendarMonthInfo returns days with image counts for a specific month (lightweight, no thumbnails)
func (s *Server) handleGetCalendarMonthInfo(c *gin.Context) {
	monthYear := c.Query("monthYear") // "YYYY-MM"
	if monthYear == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarMonthYearRequired))
		return
	}

	t, err := time.Parse(helpers.YearMonthFormat, monthYear)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidMonthYear))
		return
	}

	year := t.Year()
	month := int(t.Month())
	nextMonth := t.AddDate(0, 1, 0)

	// Get day-level counts: how many images per day in this month (PostgreSQL)
	type dayCount struct {
		Day   int `json:"day"`
		Count int `json:"count"`
	}

	var dayCounts []dayCount
	s.db.Raw(`
		SELECT 
			CAST(EXTRACT(DAY FROM date_taken) AS INTEGER) as day,
			COUNT(*) as count
		FROM image_metadata
		WHERE date_taken >= $1 AND date_taken < $2 AND date_taken IS NOT NULL
		GROUP BY EXTRACT(DAY FROM date_taken)
		ORDER BY day
	`, t, nextMonth).Scan(&dayCounts)

	// Build days array (only days that have images)
	days := make([]int, 0, len(dayCounts))
	for _, dc := range dayCounts {
		days = append(days, dc.Day)
	}

	// Get total images in this month
	var totalInMonth int
	s.db.Raw(`
		SELECT COUNT(*) FROM image_metadata
		WHERE date_taken >= $1 AND date_taken < $2 AND date_taken IS NOT NULL
	`, t, nextMonth).Scan(&totalInMonth)

	c.JSON(http.StatusOK, gin.H{
		"year":      year,
		"month":     month,
		"days":      days,
		"dayCounts": dayCounts,
		"total":     totalInMonth,
	})
}

// handleGetCalendarSeek returns a cursor pointing to a specific date
// If the requested date has no images, returns the nearest date's cursor
func (s *Server) handleGetCalendarSeek(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarMonthYearRequired))
		return
	}

	// Parse the requested date
	requestedDate, err := time.Parse(helpers.DateOnlyFormat, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgCalendarInvalidMonthYear))
		return
	}

	// Try to find the first image on this exact date
	var firstResult struct {
		ID        uint
		DateTaken time.Time
	}

	// Query: find the minimum ID for images on this date
	err = s.db.Table("image_files").
		Select("image_files.id, image_metadata.date_taken").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("DATE(image_metadata.date_taken) = ?", requestedDate).
		Order("image_files.id ASC").
		First(&firstResult).Error

	if err == nil {
		// Found images on this exact date
		c.JSON(http.StatusOK, dto.CalendarSeekResponse{
			Cursor:     helpers.EncodeCursor(firstResult.DateTaken.Format(helpers.DateOnlyFormat), firstResult.ID),
			ActualDate: firstResult.DateTaken.Format(helpers.DateOnlyFormat),
		})
		return
	}

	// No images on this date - find nearest date with images
	// Try next date first, then previous
	var nearestDate time.Time
	var nearestID uint

	// Find next date with images
	var nextResult struct {
		ID        uint
		DateTaken time.Time
	}
	err = s.db.Table("image_files").
		Select("image_files.id, image_metadata.date_taken").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken > ?", requestedDate).
		Order("image_metadata.date_taken ASC, image_files.id ASC").
		First(&nextResult).Error

	if err == nil {
		nearestDate = nextResult.DateTaken
		nearestID = nextResult.ID
	} else {
		// Try previous date
		var prevResult struct {
			ID        uint
			DateTaken time.Time
		}
		err = s.db.Table("image_files").
			Select("image_files.id, image_metadata.date_taken").
			Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
			Where("image_metadata.date_taken < ?", requestedDate).
			Order("image_metadata.date_taken DESC, image_files.id ASC").
			First(&prevResult).Error

		if err == nil {
			nearestDate = prevResult.DateTaken
			nearestID = prevResult.ID
		} else {
			// No images at all
			c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgImageNotFound))
			return
		}
	}

	c.JSON(http.StatusOK, dto.CalendarSeekResponse{
		Cursor:     helpers.EncodeCursor(nearestDate.Format(helpers.DateOnlyFormat), nearestID),
		ActualDate: nearestDate.Format(helpers.DateOnlyFormat),
	})
}

// handleGetGalleryClusters returns clustered image markers for map view
func (s *Server) handleGetGalleryClusters(c *gin.Context) {
	minLat, _ := strconv.ParseFloat(c.Query("minLat"), 64)
	maxLat, _ := strconv.ParseFloat(c.Query("maxLat"), 64)
	minLng, _ := strconv.ParseFloat(c.Query("minLng"), 64)
	maxLng, _ := strconv.ParseFloat(c.Query("maxLng"), 64)
	zoom, _ := strconv.Atoi(c.DefaultQuery("zoom", "2"))
	width, _ := strconv.Atoi(c.DefaultQuery("width", "800"))
	height, _ := strconv.Atoi(c.DefaultQuery("height", "600"))

	// Normalize latitude: clamp to [-90, 90] (Mercator projection can produce extreme values at poles)
	minLat = math.Max(-90, math.Min(90, minLat))
	maxLat = math.Max(-90, math.Min(90, maxLat))
	// Normalize longitude: wrap into [-180, 180] range (Leaflet allows dragging past date line)
	for minLng < -180 {
		minLng += 360
	}
	for minLng > 180 {
		minLng -= 360
	}
	for maxLng < -180 {
		maxLng += 360
	}
	for maxLng > 180 {
		maxLng -= 360
	}
	// Ensure proper ordering after normalization
	if minLng > maxLng {
		// If normalized bounds crossed, the view covers the whole world longitudinally
		minLng = -180
		maxLng = 180
	}
	// Ensure latitude ordering
	if minLat > maxLat {
		minLat, maxLat = maxLat, minLat
	}
	if zoom < 0 || zoom > 20 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgGeoInvalidZoom))
		return
	}
	if width <= 0 || height <= 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgGeoInvalidDimensions))
		return
	}

	params := geo.ClusterParams{
		MinLat:         minLat,
		MaxLat:         maxLat,
		MinLng:         minLng,
		MaxLng:         maxLng,
		Zoom:           zoom,
		ViewportWidth:  width,
		ViewportHeight: height,
	}

	clusters, totalImages, err := geo.ComputeClusters(s.db, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgGeoClusterFailed))
		return
	}

	// Store clusters in memory for later retrieval by clusterId
	s.clusterStorage.StoreClusters(clusters)

	// Clear ImagePaths from response (frontend will use clusterId to fetch images)
	for i := range clusters {
		clusters[i].ImagePaths = nil
	}

	c.JSON(http.StatusOK, dto.GeoClustersResponse{
		Clusters:    clusters,
		TotalImages: totalImages,
	})
}

// handleGetGeoImages returns paginated images within geographic bounds or by cluster ID
func (s *Server) handleGetGeoImages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	clusterID := c.Query("clusterId")

	// Check if clusterId is provided
	if clusterID != "" {
		s.handleGetGeoImagesByCluster(c, clusterID, page, pageSize)
		return
	}

	// Fallback to bounds-based query (for backward compatibility)
	s.handleGetGeoImagesByBounds(c, page, pageSize)
}

// handleGetGeoImagesByCluster returns images for a specific cluster
func (s *Server) handleGetGeoImagesByCluster(c *gin.Context, clusterID string, page, pageSize int) {
	// Get image paths from cluster storage
	imagePaths, found := s.clusterStorage.GetClusterImagePaths(clusterID)
	if !found {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgGeoClusterNotFound))
		return
	}

	// Calculate pagination
	totalImages := len(imagePaths)
	totalPages := (totalImages + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > totalImages {
		end = totalImages
	}

	// Get paginated paths
	paginatedPaths := imagePaths[offset:end]

	// Fetch files from database
	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.*").
		Where("image_files.path IN ?", paginatedPaths).
		Order("image_files.path").
		Find(&files)

	// Create DTOs
	pathToDTO := make(map[string]int)
	for i, path := range paginatedPaths {
		pathToDTO[path] = i
	}
	imageDTOs := make([]dto.GalleryImageDTO, len(files))
	for _, f := range files {
		if idx, ok := pathToDTO[f.Path]; ok {
			imageDTOs[idx] = dto.GalleryImageDTO{
				ID:        f.ID,
				Path:      f.Path,
				FileName:  filepath.Base(f.Path),
				DirPath:   filepath.Dir(f.Path),
				Size:      f.Size,
				SizeHuman: formatSize(f.Size),
				ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
			}
		}
	}

	// Filter out any nil entries (paths not found in DB)
	validDTOs := make([]dto.GalleryImageDTO, 0, len(imageDTOs))
	for _, dto := range imageDTOs {
		if dto.Path != "" {
			validDTOs = append(validDTOs, dto)
		}
	}
	imageDTOs = validDTOs

	// Generate thumbnails in parallel
	if len(imageDTOs) > 0 {
		paths := make([]string, len(imageDTOs))
		for i, imgDTO := range imageDTOs {
			paths[i] = imgDTO.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
	}

	c.JSON(http.StatusOK, dto.GeoImagesResponse{
		Images:      imageDTOs,
		TotalImages: totalImages,
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasNextPage: page < totalPages,
	})
}

// handleGetGeoImagesByBounds returns paginated images within geographic bounds
func (s *Server) handleGetGeoImagesByBounds(c *gin.Context, page, pageSize int) {
	minLat, _ := strconv.ParseFloat(c.Query("minLat"), 64)
	maxLat, _ := strconv.ParseFloat(c.Query("maxLat"), 64)
	minLng, _ := strconv.ParseFloat(c.Query("minLng"), 64)
	maxLng, _ := strconv.ParseFloat(c.Query("maxLng"), 64)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var totalImages int64
	s.db.Table("image_files").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.gps_latitude IS NOT NULL").
		Where("image_metadata.gps_longitude IS NOT NULL").
		Where("image_metadata.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
		Where("image_metadata.gps_longitude BETWEEN ? AND ?", minLng, maxLng).
		Count(&totalImages)

	pag := helpers.CalcPagination(page, pageSize, totalImages)
	offset := (page - 1) * pageSize

	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.*").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.gps_latitude IS NOT NULL").
		Where("image_metadata.gps_longitude IS NOT NULL").
		Where("image_metadata.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
		Where("image_metadata.gps_longitude BETWEEN ? AND ?", minLng, maxLng).
		Order("image_files.path").
		Offset(offset).
		Limit(pageSize).
		Find(&files)

	imageDTOs := make([]dto.GalleryImageDTO, len(files))
	for i, f := range files {
		imageDTOs[i] = dto.GalleryImageDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileName:  filepath.Base(f.Path),
			DirPath:   filepath.Dir(f.Path),
			Size:      f.Size,
			SizeHuman: formatSize(f.Size),
			ModTime:   f.ModTime.Format(helpers.DateTimeFormat),
		}
	}

	// Generate thumbnails in parallel
	if len(files) > 0 {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
			imageDTOs[idx].Thumbnail = thumb
		})
	}

	c.JSON(http.StatusOK, dto.GeoImagesResponse{
		Images:      imageDTOs,
		TotalImages: int(totalImages),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleGetOCRStatus returns the current OCR classifier status
func (s *Server) handleGetOCRStatus(c *gin.Context) {
	if s.ocrClient == nil || !s.config.OCREnabled {
		c.JSON(http.StatusOK, dto.OCRStatusResponse{
			Status: dto.OCRStatus{
				Enabled: false,
				Health:  "disabled",
			},
		})
		return
	}

	status := s.ocrClient.GetStatus()
	c.JSON(http.StatusOK, dto.OCRStatusResponse{
		Status: dto.OCRStatus{
			Enabled:    true,
			Health:     string(status.HealthStatus),
			LastCheck:  status.LastCheck.Format(helpers.DateTimeFormat),
			Error:      status.Error,
			ServiceURL: fmt.Sprintf("http://%s:%s", s.config.OCRHost, s.config.OCRPort),
		},
	})
}

// handleStartOcrClassification starts the OCR classification process
func (s *Server) handleStartOcrClassification(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgOcrFailed))
		return
	}

	// Default to full scan (non-incremental)
	incremental := false
	if err := s.ocrManager.StartClassification(incremental); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgOcrAlreadyRunning))
		return
	}

	c.JSON(http.StatusAccepted, dto.ScanResponse{
		Message: string(i18n.MsgOcrStarted),
	})
}

// handleStartOcrClassificationIncremental starts OCR classification for new/changed files only
func (s *Server) handleStartOcrClassificationIncremental(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgOcrFailed))
		return
	}

	if err := s.ocrManager.StartClassification(true); err != nil {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgOcrAlreadyRunning))
		return
	}

	c.JSON(http.StatusAccepted, dto.ScanResponse{
		Message: string(i18n.MsgOcrStarted),
	})
}

// handleStopOcrClassification requests a graceful stop of OCR classification
func (s *Server) handleStopOcrClassification(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgOcrFailed))
		return
	}

	if !s.ocrManager.IsProcessing() {
		c.JSON(http.StatusConflict, i18n.ErrorResponse(i18n.MsgOcrNotRunning))
		return
	}

	s.ocrManager.StopClassification()

	c.JSON(http.StatusOK, dto.ScanResponse{
		Message: "OCR classification stopping",
	})
}

// handleGetOcrClassificationStatus returns the OCR classification progress
func (s *Server) handleGetOcrClassificationStatus(c *gin.Context) {
	if s.ocrManager == nil {
		c.JSON(http.StatusOK, dto.OcrClassificationStatusResponse{
			Processing: false,
			Progress:   "OCR classification disabled",
		})
		return
	}

	status := s.ocrManager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleGetOcrDocuments returns paginated list of images classified as text documents
func (s *Server) handleGetOcrDocuments(c *gin.Context) {
	params := helpers.ParsePagination(c, helpers.ModeFixed)
	page := params.Page
	pageSize := params.PageSize
	offset := params.Offset

	// Query documents classified as text documents
	var total int64
	s.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("ocr_classifications.is_text_document = true").
		Count(&total)

	var results []struct {
		ID                 uint
		ImageFileID        uint
		Path               string
		Size               int64
		Hash               string
		ModTime            time.Time
		MeanConfidence     float32
		WeightedConfidence float32
		TokenCount         int
		Angle              int
		ScaleFactor        float32
	}

	if err := s.db.Table("ocr_classifications").
		Select("image_files.id, image_files.path, image_files.size, image_files.hash, image_files.mod_time, ocr_classifications.image_file_id, ocr_classifications.mean_confidence, ocr_classifications.weighted_confidence, ocr_classifications.token_count, ocr_classifications.angle, ocr_classifications.scale_factor").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("ocr_classifications.is_text_document = true").
		Order("image_files.id").
		Offset(offset).
		Limit(pageSize).
		Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgScanFailed))
		return
	}

	pag := helpers.CalcPagination(page, pageSize, total)

	// Build DTOs with thumbnails
	docs := make([]dto.OcrDocumentDTO, len(results))
	for i, r := range results {
		docs[i] = dto.OcrDocumentDTO{
			ID:                 r.ID,
			ImageFileID:        r.ImageFileID,
			Path:               r.Path,
			FileName:           filepath.Base(r.Path),
			DirPath:            filepath.Dir(r.Path),
			Size:               r.Size,
			SizeHuman:          formatSize(r.Size),
			ModTime:            r.ModTime.Format(helpers.DateTimeFormat),
			MeanConfidence:     r.MeanConfidence,
			WeightedConfidence: r.WeightedConfidence,
			TokenCount:         r.TokenCount,
			Angle:              r.Angle,
			ScaleFactor:        r.ScaleFactor,
		}
	}

	// Generate thumbnails in parallel
	paths := make([]string, 0, len(docs))
	pathToIdx := make(map[string]int)
	for i, doc := range docs {
		if doc.Path != "" {
			paths = append(paths, doc.Path)
			pathToIdx[doc.Path] = i
		}
	}
	s.thumbnailBatch.GenerateParallel(paths, func(idx int, thumb string) {
		docs[idx].Thumbnail = thumb
	})

	c.JSON(http.StatusOK, dto.OcrDocumentsResponse{
		Documents:   docs,
		Total:       int(total),
		CurrentPage: pag.Page,
		PageSize:    pag.PageSize,
		TotalPages:  pag.TotalPages,
		HasNextPage: pag.HasNextPage,
	})
}

// handleGetOcrData returns OCR classification data and bounding boxes for a specific image
func (s *Server) handleGetOcrData(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	// Find classification
	var classification domain.OcrClassification
	if err := s.db.Table("ocr_classifications").
		Joins("JOIN image_files ON image_files.id = ocr_classifications.image_file_id").
		Where("image_files.path = ?", imagePath).
		First(&classification).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	// Find bounding boxes
	var boxes []domain.OcrBoundingBox
	s.db.Where("classification_id = ?", classification.ID).Find(&boxes)

	// Convert to DTOs
	boxDTOs := make([]dto.BoundingBoxDTO, len(boxes))
	for i, b := range boxes {
		boxDTOs[i] = dto.BoundingBoxDTO{
			X:          b.X,
			Y:          b.Y,
			Width:      b.Width,
			Height:     b.Height,
			Word:       b.Word,
			Confidence: b.Confidence,
		}
	}

	c.JSON(http.StatusOK, dto.OcrDataResponse{
		ImagePath:         imagePath,
		Angle:             classification.Angle,
		ScaleFactor:       classification.ScaleFactor,
		IsTextDocument:    classification.IsTextDocument,
		BoundingBoxWidth:  classification.BoundingBoxWidth,
		BoundingBoxHeight: classification.BoundingBoxHeight,
		Boxes:             boxDTOs,
	})
}

// handleGetLlmSettings returns LLM settings
func (s *Server) handleGetLlmSettings(c *gin.Context) {
	settings := s.settingsLoader.LlmSettings()

	// Don't expose API key in full, mask it
	apiKey := settings.ApiKey
	if len(apiKey) > 8 {
		apiKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}

	c.JSON(http.StatusOK, dto.LlmSettingsDTO{
		ID:       settings.ID,
		Provider: settings.Provider,
		ApiUrl:   settings.ApiUrl,
		ApiKey:   apiKey,
		Model:    settings.Model,
		Enabled:  settings.Enabled,
	})
}

// handleUpdateLlmSettings updates LLM settings
func (s *Server) handleUpdateLlmSettings(c *gin.Context) {
	var req dto.UpdateLlmSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	var settings domain.LlmSettings
	err := s.db.First(&settings).Error

	if err == gorm.ErrRecordNotFound {
		// Create new settings
		settings = domain.LlmSettings{
			Provider: req.Provider,
			ApiUrl:   req.ApiUrl,
			ApiKey:   req.ApiKey,
			Model:    req.Model,
			Enabled:  req.Enabled,
		}
		if err := s.db.Create(&settings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	} else {
		// Update existing settings
		s.db.Model(&settings).Updates(map[string]interface{}{
			"provider": req.Provider,
			"api_url":  req.ApiUrl,
			"api_key":  req.ApiKey,
			"model":    req.Model,
			"enabled":  req.Enabled,
		})
	}

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleLlmRecognize starts LLM-based OCR recognition asynchronously
func (s *Server) handleLlmRecognize(c *gin.Context) {
	var req dto.LlmOcrRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Create LLM client (also validates settings and enabled state)
	llmClient, settings, ok := s.llmFactory.CreateClient(c)
	if !ok {
		return
	}

	// Get image file ID from path
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", req.ImagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	// Check if already recognized (return cached result unless force re-recognition)
	if !req.Force && s.llmOcrService != nil {
		existing, _ := s.llmOcrService.GetRecognition(imageFile.ID)
		if existing != nil && existing.Success {
			c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
				Status:           "completed",
				MarkdownContent:  existing.MarkdownContent,
				Language:         existing.Language,
				Provider:         existing.Provider,
				Model:            existing.Model,
				ProcessingTimeMs: existing.ProcessingTimeMs,
			})
			return
		}
	}

	// Start async recognition
	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return
	}

	_ = s.llmOcrService.StartRecognizeAsync(imageFile.ID, llmClient, settings)
	c.JSON(http.StatusAccepted, dto.LlmRecognizeStatusResponse{
		Status: "processing",
	})
}

// handleLlmRecognizeStatus returns the status of an async LLM recognition task
func (s *Server) handleLlmRecognizeStatus(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	// Get image file ID
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", imagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
		return
	}

	taskStatus := s.llmOcrService.GetRecognizeStatus(imageFile.ID)
	if taskStatus == nil {
		// No active task — check if there's a result in DB
		existing, _ := s.llmOcrService.GetRecognition(imageFile.ID)
		if existing != nil && existing.Success {
			c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
				Status:           "completed",
				MarkdownContent:  existing.MarkdownContent,
				Language:         existing.Language,
				Provider:         existing.Provider,
				Model:            existing.Model,
				ProcessingTimeMs: existing.ProcessingTimeMs,
			})
			return
		}
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
		return
	}

	switch taskStatus.Status {
	case "processing":
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "processing"})
	case "completed":
		resp := dto.LlmRecognizeStatusResponse{
			Status: "completed",
		}
		if taskStatus.Result != nil {
			resp.MarkdownContent = taskStatus.Result.MarkdownContent
			resp.Language = taskStatus.Result.Language
			resp.Provider = taskStatus.Result.Provider
			resp.Model = taskStatus.Result.Model
			resp.ProcessingTimeMs = taskStatus.Result.ProcessingTimeMs
		}
		c.JSON(http.StatusOK, resp)
	case "failed":
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
			Status: "failed",
			Error:  taskStatus.Error,
		})
	default:
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
	}
}

// handleGetLlmRecognition retrieves LLM OCR recognition for an image
func (s *Server) handleGetLlmRecognition(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	// Get image file ID
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", imagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	// Get recognition
	if s.llmOcrService == nil {
		c.JSON(http.StatusOK, dto.LlmOcrDataResponse{Found: false})
		return
	}

	recognition, err := s.llmOcrService.GetRecognition(imageFile.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return
	}

	if recognition == nil {
		c.JSON(http.StatusOK, dto.LlmOcrDataResponse{Found: false})
		return
	}

	c.JSON(http.StatusOK, dto.LlmOcrDataResponse{
		Found:            true,
		MarkdownContent:  recognition.MarkdownContent,
		Language:         recognition.Language,
		Provider:         recognition.Provider,
		Model:            recognition.Model,
		ProcessingTimeMs: recognition.ProcessingTimeMs,
		Success:          recognition.Success,
		Error:            recognition.Error,
		CreatedAt:        recognition.CreatedAt.Format(helpers.DateTimeFormat),
	})
}

// handleGetLlmModels returns a list of available LLM models from the configured server
func (s *Server) handleGetLlmModels(c *gin.Context) {
	// Get LLM settings
	settings, found := s.settingsLoader.LlmSettingsIfExists()
	if !found {
		c.JSON(http.StatusNotFound, dto.LlmModelsResponse{
			Success:  false,
			Error:    "LLM settings not configured",
			Provider: "",
		})
		return
	}

	if !settings.Enabled {
		c.JSON(http.StatusServiceUnavailable, dto.LlmModelsResponse{
			Success:  false,
			Error:    "LLM recognition is not enabled",
			Provider: settings.Provider,
		})
		return
	}

	// Create LLM client
	llmClient, err := llm.NewClient(settings.Provider, settings.ApiUrl, settings.ApiKey, settings.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.LlmModelsResponse{
			Success:  false,
			Error:    err.Error(),
			Provider: settings.Provider,
		})
		return
	}

	// Fetch models
	models, err := llmClient.ListModels()
	if err != nil {
		c.JSON(http.StatusOK, dto.LlmModelsResponse{
			Success:  false,
			Error:    err.Error(),
			Provider: settings.Provider,
		})
		return
	}

	// Convert to DTO
	modelDTOs := make([]dto.LlmModelDTO, len(models))
	for i, m := range models {
		modelDTOs[i] = dto.LlmModelDTO{
			ID:   m.ID,
			Name: m.Name,
			Size: m.Size,
		}
	}

	c.JSON(http.StatusOK, dto.LlmModelsResponse{
		Success:  true,
		Models:   modelDTOs,
		Provider: settings.Provider,
	})
}

// handleAiAction executes an AI action (describe, tags, recognizeText, askQuestion) asynchronously
func (s *Server) handleAiAction(c *gin.Context) {
	var req dto.AiActionRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// Create LLM client (also validates settings and enabled state)
	llmClient, _, ok := s.llmFactory.CreateClient(c)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "AI features not enabled",
		})
		return
	}

	// Get image file ID from path
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", req.ImagePath).First(&imageFile).Error; err != nil {
		c.JSON(http.StatusNotFound, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "Image not found",
		})
		return
	}

	// Validate action
	if req.Action == dto.AiActionAskQuestion && req.Question == "" {
		c.JSON(http.StatusBadRequest, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "Question is required for askQuestion action",
		})
		return
	}

	// Start async AI action
	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "AI service not available",
		})
		return
	}

	// Generate unique task ID
	taskID := uuid.New().String()

	// Start async processing
	s.llmOcrService.StartAiActionAsync(taskID, imageFile.ID, string(req.Action), req.Question, req.Language, llmClient, s.settingsLoader.LlmSettings())

	// Return 202 Accepted with task ID
	c.JSON(http.StatusAccepted, dto.AiActionStartResponse{
		TaskID: taskID,
		Action: req.Action,
		Status: "processing",
	})
}

// handleAiActionStatus returns the status of an async AI action task
func (s *Server) handleAiActionStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "Task ID is required",
		})
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "AI service not available",
		})
		return
	}

	taskStatus := s.llmOcrService.GetAiActionStatus(taskID)
	if taskStatus == nil {
		c.JSON(http.StatusNotFound, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "Task not found or expired",
		})
		return
	}

	response := dto.AiActionStatusResponse{
		TaskID: taskID,
		Status: taskStatus.Status,
		Action: dto.AiActionType(taskStatus.Action),
	}

	if taskStatus.Status == "completed" && taskStatus.Result != nil {
		response.Provider = taskStatus.Result.Provider
		response.Model = taskStatus.Result.Model
		response.ProcessingTimeMs = taskStatus.Result.ProcessingTimeMs

		if taskStatus.Action == "tags" {
			response.Tags = taskStatus.Result.Tags
		} else {
			response.Result = taskStatus.Result.Result
		}
	} else if taskStatus.Status == "failed" {
		response.Error = taskStatus.Error
	}

	c.JSON(http.StatusOK, response)
}

// handleThumbnailCacheStats возвращает статистику кэша миниатюр
func (s *Server) handleThumbnailCacheStats(c *gin.Context) {
	if s.thumbnailService == nil {
		log.Printf("Thumbnail stats requested: service is nil")
		c.JSON(http.StatusOK, thumbnail.ThumbnailStats{})
		return
	}

	stats := s.thumbnailService.Stats()
	log.Printf("Thumbnail stats: %+v", stats)
	c.JSON(http.StatusOK, stats)
}

// handleThumbnailCacheInvalidate удаляет миниатюру из кэша
func (s *Server) handleThumbnailCacheInvalidate(c *gin.Context) {
	var req dto.InvalidateThumbnailRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImagePathRequired))
		return
	}

	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailService.Invalidate(req.FilePath); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheInvalidated)
}

// handleThumbnailCacheInvalidateAll удаляет все миниатюры из кэша
func (s *Server) handleThumbnailCacheInvalidateAll(c *gin.Context) {
	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailService.InvalidateAll(); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheAllInvalidated)
}

// handleThumbnailCacheWarmup предварительно генерирует миниатюры для файлов
func (s *Server) handleThumbnailCacheWarmup(c *gin.Context) {
	var req dto.WarmupThumbnailsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgScanNoFilesSelected))
		return
	}

	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgScanDuplicateFailed))
		return
	}

	if err := s.thumbnailService.Warmup(req.FilePaths); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgImageThumbnailFailed))
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheWarmedUp)
}

// handleThumbnailCacheEnable включает кэш миниатюр
func (s *Server) handleThumbnailCacheEnable(c *gin.Context) {
	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgThumbnailCacheNotAvailable))
		return
	}

	s.thumbnailService.Enable()
	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheEnabled)
}

// handleThumbnailCacheDisable выключает кэш миниатюр
func (s *Server) handleThumbnailCacheDisable(c *gin.Context) {
	if s.thumbnailService == nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgThumbnailCacheNotAvailable))
		return
	}

	s.thumbnailService.Disable()
	s.respondSuccess(c, http.StatusOK, i18n.MsgThumbnailCacheDisabled)
}
