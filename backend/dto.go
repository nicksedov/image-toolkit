package main

// --- Duplicates API ---

// DuplicatesResponse is the JSON response for GET /api/duplicates
type DuplicatesResponse struct {
	Groups      []DuplicateGroupDTO `json:"groups"`
	TotalFiles  int                 `json:"totalFiles"`
	PageFiles   int                 `json:"pageFiles"`
	TotalGroups int                 `json:"totalGroups"`
	ScannedDirs []string            `json:"scannedDirs"`
	CurrentPage int                 `json:"currentPage"`
	PageSize    int                 `json:"pageSize"`
	TotalPages  int                 `json:"totalPages"`
	HasPrevPage bool                `json:"hasPrevPage"`
	HasNextPage bool                `json:"hasNextPage"`
	PageSizes   []int               `json:"pageSizes"`
}

// DuplicateGroupDTO represents a duplicate group in JSON responses
type DuplicateGroupDTO struct {
	Index     int       `json:"index"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	SizeHuman string    `json:"sizeHuman"`
	Files     []FileDTO `json:"files"`
	Thumbnail string    `json:"thumbnail"`
}

// FileDTO represents a file in JSON responses
type FileDTO struct {
	ID       uint   `json:"id"`
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	DirPath  string `json:"dirPath"`
	ModTime  string `json:"modTime"`
}

// --- Scan API ---

// ScanResponse is the JSON response for POST /api/scan
type ScanResponse struct {
	Message string `json:"message"`
}

// ScanStatusResponse is the JSON response for GET /api/status
type ScanStatusResponse struct {
	Scanning       bool   `json:"scanning"`
	Progress       string `json:"progress"`
	FilesProcessed int    `json:"filesProcessed"`
}

// --- Script Generation API ---

// GenerateScriptRequest represents the request for script generation
type GenerateScriptRequest struct {
	FilePaths  []string `json:"filePaths"`
	OutputDir  string   `json:"outputDir"`
	TrashDir   string   `json:"trashDir"`
	ScriptType string   `json:"scriptType"`
}

// GenerateScriptResponse represents the response from script generation
type GenerateScriptResponse struct {
	Message    string `json:"message"`
	ScriptPath string `json:"scriptPath"`
	FileCount  int    `json:"fileCount"`
}

// --- Delete Files API ---

// DeleteFilesRequest represents the request for direct file deletion
type DeleteFilesRequest struct {
	FilePaths []string `json:"filePaths"`
	TrashDir  string   `json:"trashDir"`
}

// DeleteFilesResponse represents the response from file deletion
type DeleteFilesResponse struct {
	Success     int      `json:"success"`
	Failed      int      `json:"failed"`
	FailedFiles []string `json:"failedFiles,omitempty"`
}

// --- Folder Patterns API ---

// FolderPattern represents a unique combination of folders containing duplicates
type FolderPattern struct {
	ID             string   `json:"id"`
	Folders        []string `json:"folders"`
	DuplicateCount int      `json:"duplicateCount"`
	TotalFiles     int      `json:"totalFiles"`
}

// FolderPatternsResponse represents the response for folder patterns
type FolderPatternsResponse struct {
	Patterns []FolderPattern `json:"patterns"`
}

// --- Batch Delete API ---

// BatchDeleteRequest represents a request for batch deletion
type BatchDeleteRequest struct {
	Rules    []BatchDeleteRule `json:"rules"`
	TrashDir string            `json:"trashDir"`
}

// BatchDeleteRule specifies which folder to keep for a pattern
type BatchDeleteRule struct {
	PatternID  string `json:"patternId"`
	KeepFolder string `json:"keepFolder"`
}

// BatchDeleteResponse represents the response from batch deletion
type BatchDeleteResponse struct {
	Success     int      `json:"success"`
	Failed      int      `json:"failed"`
	FailedFiles []string `json:"failedFiles,omitempty"`
}

// --- Thumbnail API ---

// ThumbnailResponse is the JSON response for GET /api/thumbnail
type ThumbnailResponse struct {
	Thumbnail string `json:"thumbnail"`
}
