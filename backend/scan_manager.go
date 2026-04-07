package main

import (
	"fmt"
	"sync"

	"gorm.io/gorm"
)

// ScanManager manages asynchronous directory scanning
type ScanManager struct {
	mu             sync.RWMutex
	isScanning     bool
	progress       string
	filesProcessed int
	db             *gorm.DB
	scanDirs       []string
}

// NewScanManager creates a new ScanManager
func NewScanManager(db *gorm.DB, scanDirs []string) *ScanManager {
	return &ScanManager{
		db:       db,
		scanDirs: scanDirs,
	}
}

// StartScan launches an asynchronous scan of all directories
func (sm *ScanManager) StartScan() error {
	sm.mu.Lock()
	if sm.isScanning {
		sm.mu.Unlock()
		return fmt.Errorf("scan already in progress")
	}
	sm.isScanning = true
	sm.progress = "Starting scan..."
	sm.filesProcessed = 0
	sm.mu.Unlock()

	go func() {
		progressChan := make(chan string, 200)

		go func() {
			count := 0
			for msg := range progressChan {
				count++
				sm.mu.Lock()
				sm.progress = msg
				sm.filesProcessed = count
				sm.mu.Unlock()
			}
		}()

		// Cleanup missing files first
		sm.mu.Lock()
		sm.progress = "Cleaning up missing files..."
		sm.mu.Unlock()
		cleanupMissingFiles(sm.db, progressChan)

		// Scan all directories
		for _, dir := range sm.scanDirs {
			sm.mu.Lock()
			sm.progress = fmt.Sprintf("Scanning: %s", dir)
			sm.mu.Unlock()
			scanDirectory(sm.db, dir, progressChan)
		}

		close(progressChan)

		sm.mu.Lock()
		sm.isScanning = false
		sm.progress = "Scan complete"
		sm.mu.Unlock()
	}()

	return nil
}

// GetStatus returns the current scan status
func (sm *ScanManager) GetStatus() ScanStatusResponse {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return ScanStatusResponse{
		Scanning:       sm.isScanning,
		Progress:       sm.progress,
		FilesProcessed: sm.filesProcessed,
	}
}

// IsScanning returns whether a scan is currently running
func (sm *ScanManager) IsScanning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isScanning
}
