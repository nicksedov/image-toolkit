package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/charmap"
	"gorm.io/gorm"
)

// Server holds the application state
type Server struct {
	db             *gorm.DB
	thumbnailCache *ThumbnailCache
	scanManager    *ScanManager
	config         *AppConfig
}

// NewServer creates a new server instance
func NewServer(db *gorm.DB, scanManager *ScanManager, config *AppConfig) *Server {
	return &Server{
		db:             db,
		thumbnailCache: NewThumbnailCache(),
		scanManager:    scanManager,
		config:         config,
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

// handleGetDuplicates returns paginated duplicate groups as JSON
func (s *Server) handleGetDuplicates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

	validPageSizes := []int{50, 100, 250, 500}
	isValidPageSize := false
	for _, ps := range validPageSizes {
		if pageSize == ps {
			isValidPageSize = true
			break
		}
	}
	if !isValidPageSize {
		pageSize = 50
	}

	if page < 1 {
		page = 1
	}

	offset := (page - 1) * pageSize
	groups, totalGroups, totalFiles, err := findDuplicatesPaginated(s.db, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to find duplicates: %v", err)})
		return
	}

	totalPages := (totalGroups + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// Prepare group DTOs with parallel thumbnail generation
	groupDTOs := make([]DuplicateGroupDTO, len(groups))
	pageFiles := 0

	for _, g := range groups {
		pageFiles += len(g.Files)
	}

	const maxWorkers = 16
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)

	for i, g := range groups {
		fileDTOs := make([]FileDTO, len(g.Files))
		for j, f := range g.Files {
			fileDTOs[j] = FileDTO{
				ID:       f.ID,
				Path:     f.Path,
				FileName: filepath.Base(f.Path),
				DirPath:  filepath.Dir(f.Path),
				ModTime:  f.ModTime.Format("2006-01-02 15:04:05"),
			}
		}

		groupDTOs[i] = DuplicateGroupDTO{
			Index:     offset + i + 1,
			Hash:      g.Hash,
			Size:      g.Size,
			SizeHuman: formatSize(g.Size),
			Files:     fileDTOs,
		}

		if len(g.Files) > 0 {
			wg.Add(1)
			go func(idx int, filePath string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				thumb, err := generateThumbnail(filePath, s.thumbnailCache)
				if err == nil {
					groupDTOs[idx].Thumbnail = thumb
				}
			}(i, g.Files[0].Path)
		}
	}

	wg.Wait()

	// Get scanned dirs from gallery folders
	var galleryFolders []GalleryFolder
	s.db.Order("created_at").Find(&galleryFolders)
	scannedDirs := make([]string, len(galleryFolders))
	for i, f := range galleryFolders {
		scannedDirs[i] = f.Path
	}

	response := DuplicatesResponse{
		Groups:      groupDTOs,
		TotalFiles:  totalFiles,
		PageFiles:   pageFiles,
		TotalGroups: totalGroups,
		ScannedDirs: scannedDirs,
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasPrevPage: page > 1,
		HasNextPage: page < totalPages,
		PageSizes:   validPageSizes,
	}

	c.JSON(http.StatusOK, response)
}

// handleScan triggers an async scan of directories
func (s *Server) handleScan(c *gin.Context) {
	if err := s.scanManager.StartScan(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, ScanResponse{Message: "Scan started"})
}

// handleGetStatus returns the current scan status
func (s *Server) handleGetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.scanManager.GetStatus())
}

// handleGenerateScript generates a script for moving files
func (s *Server) handleGenerateScript(c *gin.Context) {
	var req GenerateScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files selected"})
		return
	}

	if req.OutputDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Output directory not specified"})
		return
	}

	if req.TrashDir == "" {
		req.TrashDir = filepath.Join(req.OutputDir, "trash")
	}

	if req.ScriptType == "" {
		req.ScriptType = "bash"
	}

	var script string
	var scriptPath string
	var scriptBytes []byte

	if req.ScriptType == "windows" {
		windowsPaths := make([]string, len(req.FilePaths))
		for i, p := range req.FilePaths {
			windowsPaths[i] = strings.ReplaceAll(p, "/", "\\")
		}
		windowsTrashDir := strings.ReplaceAll(req.TrashDir, "/", "\\")

		script = generateWindowsScript(windowsPaths, windowsTrashDir)
		scriptPath = filepath.Join(req.OutputDir, "remove_duplicates.ps1")

		encoder := charmap.Windows1251.NewEncoder()
		encoded, err := encoder.String(script)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to encode script: %v", err)})
			return
		}
		scriptBytes = []byte(encoded)
	} else {
		script = generateBashScript(req.FilePaths, req.TrashDir)
		scriptPath = filepath.Join(req.OutputDir, "remove_duplicates.sh")
		scriptBytes = []byte(script)
	}

	if err := os.WriteFile(scriptPath, scriptBytes, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save script: %v", err)})
		return
	}

	c.JSON(http.StatusOK, GenerateScriptResponse{
		Message:    "Script generated successfully",
		ScriptPath: scriptPath,
		FileCount:  len(req.FilePaths),
	})
}

// generateBashScript creates a bash script for Unix/Linux/macOS
func generateBashScript(filePaths []string, trashDir string) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n\n")
	sb.WriteString("# Image Dedup - File Removal Script\n")
	sb.WriteString(fmt.Sprintf("# Generated at: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("# Files to move: %d\n\n", len(filePaths)))

	sb.WriteString("# Create trash directory\n")
	sb.WriteString(fmt.Sprintf("TRASH_DIR=\"%s\"\n", trashDir))
	sb.WriteString("mkdir -p \"$TRASH_DIR\"\n\n")

	sb.WriteString("# Move files to trash\n")
	for _, path := range filePaths {
		escapedPath := strings.ReplaceAll(path, "\"", "\\\"")
		escapedPath = strings.ReplaceAll(escapedPath, "$", "\\$")

		baseName := filepath.Base(path)
		sb.WriteString(fmt.Sprintf("mv \"%s\" \"$TRASH_DIR/%s\" 2>/dev/null && echo \"Moved: %s\" || echo \"Failed: %s\"\n",
			escapedPath, baseName, baseName, baseName))
	}

	sb.WriteString("\necho \"Done! Moved files are in: $TRASH_DIR\"\n")
	return sb.String()
}

// generateWindowsScript creates a PowerShell script for Windows
func generateWindowsScript(filePaths []string, trashDir string) string {
	var sb strings.Builder
	sb.WriteString("# Image Dedup - File Removal Script (PowerShell)\n")
	sb.WriteString(fmt.Sprintf("# Generated at: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("# Files to move: %d\n\n", len(filePaths)))

	escapedTrashDir := strings.ReplaceAll(trashDir, "'", "''")
	sb.WriteString("# Create trash directory\n")
	sb.WriteString(fmt.Sprintf("$TrashDir = '%s'\n", escapedTrashDir))
	sb.WriteString("if (-not (Test-Path -Path $TrashDir)) {\n")
	sb.WriteString("    New-Item -ItemType Directory -Path $TrashDir -Force | Out-Null\n")
	sb.WriteString("}\n\n")

	sb.WriteString("# Move files to trash\n")
	for _, path := range filePaths {
		escapedPath := strings.ReplaceAll(path, "'", "''")
		baseName := filepath.Base(path)
		escapedBaseName := strings.ReplaceAll(baseName, "'", "''")

		sb.WriteString("try {\n")
		sb.WriteString(fmt.Sprintf("    Move-Item -Path '%s' -Destination (Join-Path $TrashDir '%s') -Force\n", escapedPath, escapedBaseName))
		sb.WriteString(fmt.Sprintf("    Write-Host \"Moved: %s\" -ForegroundColor Green\n", baseName))
		sb.WriteString("} catch {\n")
		sb.WriteString(fmt.Sprintf("    Write-Host \"Failed: %s - $_\" -ForegroundColor Red\n", baseName))
		sb.WriteString("}\n\n")
	}

	sb.WriteString("Write-Host \"\"\n")
	sb.WriteString("Write-Host \"Done! Moved files are in: $TrashDir\" -ForegroundColor Cyan\n")
	sb.WriteString("Write-Host \"Press any key to exit...\"\n")
	sb.WriteString("$null = $Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')\n")
	return sb.String()
}

// handleThumbnail serves a thumbnail for a specific file
func (s *Server) handleThumbnail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path required"})
		return
	}

	thumbnail, err := generateThumbnail(path, s.thumbnailCache)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate thumbnail: %v", err)})
		return
	}

	c.JSON(http.StatusOK, ThumbnailResponse{Thumbnail: thumbnail})
}

// handleDeleteFiles deletes selected files directly (moves to trash)
func (s *Server) handleDeleteFiles(c *gin.Context) {
	var req DeleteFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files selected"})
		return
	}

	var successCount, failedCount int
	var failedFiles []string

	if req.TrashDir != "" {
		if err := os.MkdirAll(req.TrashDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trash directory: " + err.Error()})
			return
		}

		for _, filePath := range req.FilePaths {
			baseName := filepath.Base(filePath)
			destPath := filepath.Join(req.TrashDir, baseName)

			if _, err := os.Stat(destPath); err == nil {
				ext := filepath.Ext(baseName)
				nameWithoutExt := strings.TrimSuffix(baseName, ext)
				destPath = filepath.Join(req.TrashDir, nameWithoutExt+"_"+time.Now().Format("20060102_150405")+ext)
			}

			if err := os.Rename(filePath, destPath); err != nil {
				failedCount++
				failedFiles = append(failedFiles, baseName+": "+err.Error())
				continue
			}

			s.db.Where("path = ?", filepath.ToSlash(filePath)).Delete(&ImageFile{})
			successCount++
		}
	} else {
		for _, filePath := range req.FilePaths {
			baseName := filepath.Base(filePath)

			if err := os.Remove(filePath); err != nil {
				failedCount++
				failedFiles = append(failedFiles, baseName+": "+err.Error())
				continue
			}

			s.db.Where("path = ?", filepath.ToSlash(filePath)).Delete(&ImageFile{})
			successCount++
		}
	}

	c.JSON(http.StatusOK, DeleteFilesResponse{
		Success:     successCount,
		Failed:      failedCount,
		FailedFiles: failedFiles,
	})
}

// handleGetFolderPatterns returns all unique folder patterns from duplicates
func (s *Server) handleGetFolderPatterns(c *gin.Context) {
	groups, _, _, err := findDuplicatesPaginated(s.db, 0, 100000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find duplicates: " + err.Error()})
		return
	}

	patternMap := make(map[string]*FolderPattern)

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

		sortStrings(folders)

		patternID := createPatternID(folders)

		if existing, ok := patternMap[patternID]; ok {
			existing.DuplicateCount++
			existing.TotalFiles += len(group.Files)
		} else {
			patternMap[patternID] = &FolderPattern{
				ID:             patternID,
				Folders:        folders,
				DuplicateCount: 1,
				TotalFiles:     len(group.Files),
			}
		}
	}

	patterns := make([]FolderPattern, 0, len(patternMap))
	for _, p := range patternMap {
		patterns = append(patterns, *p)
	}

	sortPatternsByCount(patterns)

	c.JSON(http.StatusOK, FolderPatternsResponse{Patterns: patterns})
}

// sortStrings sorts a slice of strings in place
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// sortPatternsByCount sorts patterns by duplicate count descending
func sortPatternsByCount(patterns []FolderPattern) {
	for i := 0; i < len(patterns)-1; i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[i].DuplicateCount < patterns[j].DuplicateCount {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}
}

// createPatternID creates a unique ID from sorted folder paths
func createPatternID(folders []string) string {
	return strings.Join(folders, "|")
}

// handleBatchDelete applies batch deletion rules to all matching duplicates
func (s *Server) handleBatchDelete(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Rules) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No rules specified"})
		return
	}

	ruleMap := make(map[string]string)
	for _, rule := range req.Rules {
		ruleMap[rule.PatternID] = rule.KeepFolder
	}

	groups, _, _, err := findDuplicatesPaginated(s.db, 0, 100000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find duplicates: " + err.Error()})
		return
	}

	var successCount, failedCount int
	var failedFiles []string

	if req.TrashDir != "" {
		if err := os.MkdirAll(req.TrashDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trash directory: " + err.Error()})
			return
		}
	}

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
		sortStrings(folders)

		patternID := createPatternID(folders)

		keepFolder, hasRule := ruleMap[patternID]
		if !hasRule {
			continue
		}

		for _, file := range group.Files {
			fileDir := filepath.Dir(file.Path)
			if fileDir == keepFolder {
				continue
			}

			if req.TrashDir != "" {
				baseName := filepath.Base(file.Path)
				destPath := filepath.Join(req.TrashDir, baseName)

				if _, err := os.Stat(destPath); err == nil {
					ext := filepath.Ext(baseName)
					nameWithoutExt := strings.TrimSuffix(baseName, ext)
					destPath = filepath.Join(req.TrashDir, nameWithoutExt+"_"+time.Now().Format("20060102_150405_000")+ext)
				}

				if err := os.Rename(file.Path, destPath); err != nil {
					failedCount++
					failedFiles = append(failedFiles, filepath.Base(file.Path)+": "+err.Error())
					continue
				}
			} else {
				if err := os.Remove(file.Path); err != nil {
					failedCount++
					failedFiles = append(failedFiles, filepath.Base(file.Path)+": "+err.Error())
					continue
				}
			}

			s.db.Where("path = ?", filepath.ToSlash(file.Path)).Delete(&ImageFile{})
			successCount++
		}
	}

	c.JSON(http.StatusOK, BatchDeleteResponse{
		Success:     successCount,
		Failed:      failedCount,
		FailedFiles: failedFiles,
	})
}

// --- Gallery Folder Handlers ---

// handleGetFolders returns all gallery folders
func (s *Server) handleGetFolders(c *gin.Context) {
	var folders []GalleryFolder
	s.db.Order("created_at").Find(&folders)

	folderDTOs := make([]GalleryFolderDTO, len(folders))
	for i, f := range folders {
		var count int64
		prefix := f.Path + "/"
		s.db.Model(&ImageFile{}).Where("path LIKE ?", prefix+"%").Count(&count)

		folderDTOs[i] = GalleryFolderDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileCount: int(count),
			CreatedAt: f.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	c.JSON(http.StatusOK, GalleryFoldersResponse{
		Folders:      folderDTOs,
		TotalFolders: len(folderDTOs),
	})
}

// handleAddFolder adds a new gallery folder and triggers a scan
func (s *Server) handleAddFolder(c *gin.Context) {
	var req AddFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is required"})
		return
	}

	// Validate directory exists
	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid path: %v", err)})
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Cannot access path: %v", err)})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is not a directory"})
		return
	}

	normalizedPath := filepath.ToSlash(absPath)

	folder := GalleryFolder{Path: normalizedPath}
	if result := s.db.Create(&folder); result.Error != nil {
		if strings.Contains(result.Error.Error(), "duplicate") || strings.Contains(result.Error.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, gin.H{"error": "This folder is already in the gallery"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to add folder: %v", result.Error)})
		return
	}

	// Trigger background scan for this folder
	scanStarted := true
	if err := s.scanManager.ScanSingleDir(normalizedPath); err != nil {
		scanStarted = false
	}

	c.JSON(http.StatusOK, AddFolderResponse{
		Message: "Folder added to gallery",
		Folder: GalleryFolderDTO{
			ID:        folder.ID,
			Path:      folder.Path,
			FileCount: 0,
			CreatedAt: folder.CreatedAt.Format("2006-01-02 15:04:05"),
		},
		ScanStarted: scanStarted,
	})
}

// handleRemoveFolder removes a gallery folder and its files from the database
func (s *Server) handleRemoveFolder(c *gin.Context) {
	id := c.Param("id")

	var folder GalleryFolder
	if result := s.db.First(&folder, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Folder not found"})
		return
	}

	// Delete all image files under this folder
	prefix := folder.Path + "/"
	result := s.db.Where("path LIKE ?", prefix+"%").Delete(&ImageFile{})
	filesRemoved := int(result.RowsAffected)

	// Delete the folder record
	s.db.Delete(&folder)

	c.JSON(http.StatusOK, RemoveFolderResponse{
		Message:      "Folder removed from gallery",
		FilesRemoved: filesRemoved,
	})
}

// handleGetGalleryImages returns paginated gallery images
func (s *Server) handleGetGalleryImages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	view := c.DefaultQuery("view", "list")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var totalImages int64
	s.db.Model(&ImageFile{}).Count(&totalImages)

	totalPages := (int(totalImages) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize

	var files []ImageFile
	s.db.Order("path").Offset(offset).Limit(pageSize).Find(&files)

	imageDTOs := make([]GalleryImageDTO, len(files))
	for i, f := range files {
		imageDTOs[i] = GalleryImageDTO{
			ID:        f.ID,
			Path:      f.Path,
			FileName:  filepath.Base(f.Path),
			DirPath:   filepath.Dir(f.Path),
			Size:      f.Size,
			SizeHuman: formatSize(f.Size),
			ModTime:   f.ModTime.Format("2006-01-02 15:04:05"),
		}
	}

	// Generate thumbnails in parallel if thumbnail view
	if view == "thumbnails" && len(files) > 0 {
		const maxWorkers = 16
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, maxWorkers)

		for i, f := range files {
			wg.Add(1)
			go func(idx int, filePath string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				thumb, err := generateThumbnail(filePath, s.thumbnailCache)
				if err == nil {
					imageDTOs[idx].Thumbnail = thumb
				}
			}(i, f.Path)
		}
		wg.Wait()
	}

	c.JSON(http.StatusOK, GalleryImagesResponse{
		Images:      imageDTOs,
		TotalImages: int(totalImages),
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasNextPage: page < totalPages,
	})
}

// handleServeImage serves a full-size image file
func (s *Server) handleServeImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path required"})
		return
	}

	// Security: verify the path is within a gallery folder
	var folders []GalleryFolder
	s.db.Find(&folders)

	allowed := false
	for _, f := range folders {
		if strings.HasPrefix(path, f.Path+"/") || strings.HasPrefix(path, f.Path+"\\") {
			allowed = true
			break
		}
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: path is not within a gallery folder"})
		return
	}

	// Convert slash path to OS path for file serving
	osPath := filepath.FromSlash(path)

	if _, err := os.Stat(osPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(osPath)
}

// SetupRouter sets up the Gin router with all API routes
func (s *Server) SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS middleware
	r.Use(SetupCORS(s.config))

	// API routes
	api := r.Group("/api")
	{
		api.GET("/duplicates", s.handleGetDuplicates)
		api.POST("/scan", s.handleScan)
		api.GET("/status", s.handleGetStatus)
		api.POST("/generate-script", s.handleGenerateScript)
		api.POST("/delete-files", s.handleDeleteFiles)
		api.GET("/thumbnail", s.handleThumbnail)
		api.GET("/folder-patterns", s.handleGetFolderPatterns)
		api.POST("/batch-delete", s.handleBatchDelete)
		api.GET("/folders", s.handleGetFolders)
		api.POST("/folders", s.handleAddFolder)
		api.DELETE("/folders/:id", s.handleRemoveFolder)
		api.GET("/gallery", s.handleGetGalleryImages)
		api.GET("/image", s.handleServeImage)
	}

	return r
}
