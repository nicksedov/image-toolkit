package imaging

import (
	"fmt"
	"log"
	"sync"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// TagScanProgress holds the current scanning progress
type TagScanProgress struct {
	Total        int
	Scanned      int
	Remaining    int
	CurrentImage string
	LastError    string
}

// TagScanStatus holds the current status of the tag scan manager
type TagScanStatus struct {
	Running  bool
	Paused   bool
	Enabled  bool
	Schedule string
	Progress TagScanProgress
}

// TagScanManager manages background tag scanning of gallery images
type TagScanManager struct {
	mu            sync.Mutex
	running       bool
	paused        bool
	stopCh        chan struct{}
	scheduleCh    chan struct{}
	pauseCh       chan struct{}
	resumeCh      chan struct{}
	pauseAckCh    chan struct{}
	pauseDepth    int
	db            *gorm.DB
	llmOcrService *LlmOcrService
	enabled       bool
	startHour     int
	startMinute   int
	endHour       int
	endMinute     int
	cursor        uint
	progress      TagScanProgress
}

// NewTagScanManager creates a new tag scan manager
func NewTagScanManager(db *gorm.DB, llmOcrService *LlmOcrService) *TagScanManager {
	return &TagScanManager{
		db:            db,
		llmOcrService: llmOcrService,
		enabled:       true,
		startHour:     22,
		startMinute:   0,
		endHour:       7,
		endMinute:     0,
		stopCh:        make(chan struct{}),
		scheduleCh:    make(chan struct{}),
	}
}

// Start begins the tag scanning loop with the given schedule
func (tsm *TagScanManager) Start(enabled bool, startH, startM, endH, endM int) {
	tsm.mu.Lock()
	if tsm.running {
		tsm.mu.Unlock()
		log.Println("Tag scanning already running")
		return
	}
	tsm.running = true
	tsm.enabled = enabled
	tsm.startHour = startH
	tsm.startMinute = startM
	tsm.endHour = endH
	tsm.endMinute = endM
	tsm.stopCh = make(chan struct{})
	tsm.scheduleCh = make(chan struct{})
	tsm.mu.Unlock()

	log.Printf("Starting background tag scanning (window %02d:%02d - %02d:%02d, enabled=%v)", startH, startM, endH, endM, enabled)
	go tsm.scheduleLoop()
}

// Stop stops the tag scanning
func (tsm *TagScanManager) Stop() {
	tsm.mu.Lock()
	if !tsm.running {
		tsm.mu.Unlock()
		return
	}
	tsm.running = false
	close(tsm.stopCh)
	tsm.mu.Unlock()

	log.Println("Background tag scanning stopped")
}

// IsRunning returns whether the tag scanning is currently running
func (tsm *TagScanManager) IsRunning() bool {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	return tsm.running
}

// UpdateSchedule updates the schedule at runtime and restarts the loop
func (tsm *TagScanManager) UpdateSchedule(enabled bool, startH, startM, endH, endM int) {
	tsm.mu.Lock()
	wasRunning := tsm.running
	tsm.enabled = enabled
	tsm.startHour = startH
	tsm.startMinute = startM
	tsm.endHour = endH
	tsm.endMinute = endM
	tsm.mu.Unlock()

	log.Printf("Tag scanning schedule updated: window %02d:%02d - %02d:%02d, enabled=%v", startH, startM, endH, endM, enabled)

	if wasRunning {
		select {
		case tsm.scheduleCh <- struct{}{}:
		default:
		}
	}
}

// RequestPause requests the scanner to pause (for AI task coordination)
func (tsm *TagScanManager) RequestPause() {
	tsm.mu.Lock()
	if !tsm.running {
		tsm.mu.Unlock()
		return
	}
	tsm.pauseDepth++
	if tsm.pauseDepth > 1 {
		// Already paused, just increment counter
		tsm.mu.Unlock()
		return
	}
	// First pause request - initialize channels
	tsm.pauseCh = make(chan struct{})
	tsm.resumeCh = make(chan struct{})
	tsm.pauseAckCh = make(chan struct{})
	tsm.mu.Unlock()

	// Signal pause
	select {
	case tsm.pauseCh <- struct{}{}:
		// Wait for acknowledgment
		<-tsm.pauseAckCh
	case <-time.After(5 * time.Second):
		log.Println("Tag scan pause request timed out")
	}
}

// Resume resumes the scanner after an AI task completes
func (tsm *TagScanManager) Resume() {
	tsm.mu.Lock()
	if tsm.pauseDepth <= 0 {
		tsm.mu.Unlock()
		return
	}
	tsm.pauseDepth--
	shouldResume := tsm.pauseDepth == 0
	tsm.mu.Unlock()

	if shouldResume {
		select {
		case tsm.resumeCh <- struct{}{}:
		default:
		}
	}
}

// GetStatus returns the current tag scan status
func (tsm *TagScanManager) GetStatus() TagScanStatus {
	tsm.mu.Lock()
	running := tsm.running
	paused := tsm.paused
	enabled := tsm.enabled
	schedule := fmt.Sprintf("%02d:%02d - %02d:%02d", tsm.startHour, tsm.startMinute, tsm.endHour, tsm.endMinute)
	progress := tsm.progress
	tsm.mu.Unlock()

	// If the manager is running but progress hasn't been initialized yet
	// (i.e. waiting for scan window to open), query the DB for untagged count
	if running && progress.Total == 0 {
		var total int64
		tsm.db.Table("image_files").
			Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
			Where("image_tags.id IS NULL").
			Count(&total)
		progress.Total = int(total)
		progress.Remaining = int(total)
	}

	return TagScanStatus{
		Running:  running,
		Paused:   paused,
		Enabled:  enabled,
		Schedule: schedule,
		Progress: progress,
	}
}

// scheduleLoop runs the tag scanning within the configured time window
func (tsm *TagScanManager) scheduleLoop() {
	for {
		tsm.mu.Lock()
		enabled := tsm.enabled
		stopCh := tsm.stopCh
		tsm.mu.Unlock()

		if !enabled {
			// Wait for schedule change or stop
			select {
			case <-stopCh:
				return
			case <-tsm.scheduleCh:
				continue
			}
		}

		// If currently inside the scanning window, start scanning immediately
		tsm.mu.Lock()
		inWindow := isWithinWindow(tsm.startHour, tsm.startMinute, tsm.endHour, tsm.endMinute)
		tsm.mu.Unlock()

		if inWindow {
			tsm.scanWindow()
			// After scanWindow completes, the window may still be open but we've
			// finished a pass. Wait briefly then re-check to avoid a tight loop
			// while still being responsive to schedule changes.
			select {
			case <-time.After(30 * time.Second):
			case <-tsm.stopCh:
				return
			case <-tsm.scheduleCh:
			}
			continue
		}

		// Outside the window - calculate when it next opens
		nextWindowOpen := tsm.calculateNextWindowOpen()
		log.Printf("Tag scan: next window opens at %s", nextWindowOpen.Format("15:04:05"))

		select {
		case <-time.After(time.Until(nextWindowOpen)):
			// Window opened, start scanning
			tsm.scanWindow()
		case <-stopCh:
			return
		case <-tsm.scheduleCh:
			// Schedule updated, restart the loop
			continue
		}
	}
}

// calculateNextWindowOpen calculates when the scanning window next opens.
// This should only be called when we are currently OUTSIDE the window.
func (tsm *TagScanManager) calculateNextWindowOpen() time.Time {
	tsm.mu.Lock()
	startH, startM := tsm.startHour, tsm.startMinute
	tsm.mu.Unlock()

	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), startH, startM, 0, 0, now.Location())

	// If the start time has already passed today, schedule for tomorrow
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	return next
}

// scanWindow runs scanning while within the configured time window
func (tsm *TagScanManager) scanWindow() {
	// Count total untagged images
	var total int64
	tsm.db.Table("image_files").
		Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
		Where("image_tags.id IS NULL").
		Count(&total)

	if total == 0 {
		log.Println("Tag scan: all images already tagged")
		return
	}

	tsm.mu.Lock()
	tsm.progress = TagScanProgress{
		Total:     int(total),
		Scanned:   0,
		Remaining: int(total),
	}
	tsm.mu.Unlock()

	log.Printf("Tag scan: starting window scan, %d untagged images", total)

	for {
		// Check if still within window — read fields under lock, then call
		// isWithinWindow() outside the lock to avoid deadlock (Go sync.Mutex
		// is not reentrant).
		tsm.mu.Lock()
		startH, startM := tsm.startHour, tsm.startMinute
		endH, endM := tsm.endHour, tsm.endMinute
		stopCh := tsm.stopCh
		pauseCh := tsm.pauseCh
		resumeCh := tsm.resumeCh
		pauseAckCh := tsm.pauseAckCh
		tsm.mu.Unlock()

		if !isWithinWindow(startH, startM, endH, endM) {
			log.Println("Tag scan: window closed, stopping scan")
			break
		}

		// Check for pause request
		select {
		case <-pauseCh:
			tsm.mu.Lock()
			tsm.paused = true
			tsm.mu.Unlock()
			log.Println("Tag scan: paused")
			// Acknowledge pause
			select {
			case pauseAckCh <- struct{}{}:
			default:
			}
			// Wait for resume
			<-resumeCh
			tsm.mu.Lock()
			tsm.paused = false
			tsm.mu.Unlock()
			log.Println("Tag scan: resumed")
			continue
		default:
		}

		// Check for stop
		select {
		case <-stopCh:
			log.Println("Tag scan: stopped during window scan")
			return
		default:
		}

		// Find next untagged image
		var imageFile domain.ImageFile
		err := tsm.db.Table("image_files").
			Select("image_files.*").
			Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
			Where("image_tags.id IS NULL AND image_files.id > ?", tsm.cursor).
			Order("image_files.id ASC").
			First(&imageFile).Error

		if err != nil {
			// No more untagged images
			log.Println("Tag scan: no more untagged images found")
			break
		}

		// Update progress
		tsm.mu.Lock()
		tsm.progress.CurrentImage = imageFile.Path
		tsm.mu.Unlock()

		// Process image
		tsm.processImage(imageFile)

		// Update cursor and progress
		tsm.mu.Lock()
		tsm.cursor = imageFile.ID
		tsm.progress.Scanned++
		tsm.progress.Remaining--
		tsm.mu.Unlock()
	}

	log.Printf("Tag scan: window scan complete, %d images scanned", tsm.progress.Scanned)
}

// isWithinWindowNow checks if the current time is within the scanning window.
// Only for external use (e.g. GetStatus). Must NOT be called while holding tsm.mu.
func (tsm *TagScanManager) isWithinWindowNow() bool {
	tsm.mu.Lock()
	startH, startM, endH, endM := tsm.startHour, tsm.startMinute, tsm.endHour, tsm.endMinute
	tsm.mu.Unlock()

	return isWithinWindow(startH, startM, endH, endM)
}

// isWithinWindow checks if the given time (hours, minutes) is within the scanning window
func isWithinWindow(startH, startM, endH, endM int) bool {
	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	if startMinutes == endMinutes {
		// Same time means 24-hour window
		return true
	}

	if startMinutes < endMinutes {
		// Normal window (e.g., 09:00 - 17:00)
		return currentMinutes >= startMinutes && currentMinutes <= endMinutes
	}

	// Overnight window (e.g., 22:00 - 07:00)
	return currentMinutes >= startMinutes || currentMinutes <= endMinutes
}

// processImage processes a single image, generating tags via LLM
func (tsm *TagScanManager) processImage(imageFile domain.ImageFile) {
	// Check if LLM is enabled
	var settings domain.LlmSettings
	if err := tsm.db.First(&settings).Error; err != nil {
		log.Printf("Tag scan: failed to load LLM settings: %v", err)
		tsm.mu.Lock()
		tsm.progress.LastError = "Failed to load LLM settings"
		tsm.mu.Unlock()
		return
	}

	if !settings.Enabled {
		log.Println("Tag scan: LLM not enabled, skipping")
		return
	}

	// Create LLM client
	client, err := llm.NewClient(settings.Provider, settings.ApiUrl, settings.ApiKey, settings.Model)
	if err != nil {
		log.Printf("Tag scan: failed to create LLM client: %v", err)
		tsm.mu.Lock()
		tsm.progress.LastError = fmt.Sprintf("Failed to create LLM client: %v", err)
		tsm.mu.Unlock()
		return
	}

	// Execute AI action "tags"
	result, err := tsm.llmOcrService.ExecuteAiAction(imageFile.ID, "tags", "", "en", client, settings)
	if err != nil {
		log.Printf("Tag scan: failed to generate tags for %s: %v", imageFile.Path, err)
		tsm.mu.Lock()
		tsm.progress.LastError = fmt.Sprintf("Failed to tag %s: %v", imageFile.Path, err)
		tsm.mu.Unlock()
		return
	}

	// Save tags: delete existing tags first, then insert new ones
	tsm.db.Where("image_file_id = ?", imageFile.ID).Delete(&domain.ImageTag{})

	if len(result.Tags) > 0 {
		tags := make([]domain.ImageTag, len(result.Tags))
		for i, tag := range result.Tags {
			tags[i] = domain.ImageTag{
				ImageFileID: imageFile.ID,
				Tag:         tag,
			}
		}
		if err := tsm.db.Create(&tags).Error; err != nil {
			log.Printf("Tag scan: failed to save tags for %s: %v", imageFile.Path, err)
			tsm.mu.Lock()
			tsm.progress.LastError = fmt.Sprintf("Failed to save tags for %s: %v", imageFile.Path, err)
			tsm.mu.Unlock()
			return
		}
		log.Printf("Tag scan: saved %d tags for %s", len(result.Tags), imageFile.Path)
	}
}
