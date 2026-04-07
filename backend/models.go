package main

import (
	"path/filepath"
	"strings"
	"time"
)

// ImageFile represents an image file in the database
type ImageFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	Size      int64     `gorm:"not null;index:idx_size_hash" json:"size"`
	Hash      string    `gorm:"not null;index:idx_size_hash" json:"hash"`
	ModTime   time.Time `gorm:"not null" json:"modTime"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DuplicateGroup represents a group of duplicate images
type DuplicateGroup struct {
	Hash  string
	Size  int64
	Files []ImageFile
}

// supportedExtensions contains all supported image file extensions
var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".webp": true,
}

// isImageFile checks if a file is a supported image based on extension
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

// GalleryFolder represents a configured gallery folder in the database
type GalleryFolder struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AppSettings stores global application settings (singleton, ID=1)
type AppSettings struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Theme     string    `gorm:"default:light;not null" json:"theme"`
	Language  string    `gorm:"default:en;not null" json:"language"`
	TrashDir  string    `gorm:"default:''" json:"trashDir"`
	UpdatedAt time.Time `json:"updatedAt"`
}
