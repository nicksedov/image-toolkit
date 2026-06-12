package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Domain models (local copies — standalone app does not import main backend)
// ---------------------------------------------------------------------------

// ImageFile mirrors the image_files table.
type ImageFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	Size      int64     `gorm:"not null" json:"size"`
	Hash      string    `gorm:"not null" json:"hash"`
	ModTime   time.Time `gorm:"not null" json:"modTime"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ImageMetadata mirrors the image_metadata table.
type ImageMetadata struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ImageFileID    uint       `gorm:"uniqueIndex;not null" json:"imageFileId"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	CameraModel    string     `json:"cameraModel"`
	LensModel      string     `json:"lensModel"`
	ISO            int        `json:"iso"`
	Aperture       string     `json:"aperture"`
	ShutterSpeed   string     `json:"shutterSpeed"`
	FocalLength    string     `json:"focalLength"`
	DateTaken      *time.Time `json:"dateTaken"`
	Orientation    int        `json:"orientation"`
	ColorSpace     string     `json:"colorSpace"`
	Software       string     `json:"software"`
	GeolocationRef *uint      `gorm:"index" json:"geolocationRef"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// ImageMetadataNew mirrors image_metadata_new — same schema, separate table.
// Used for files that have no row in image_metadata.
type ImageMetadataNew struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ImageFileID    uint       `gorm:"uniqueIndex;not null" json:"imageFileId"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	CameraModel    string     `json:"cameraModel"`
	LensModel      string     `json:"lensModel"`
	ISO            int        `json:"iso"`
	Aperture       string     `json:"aperture"`
	ShutterSpeed   string     `json:"shutterSpeed"`
	FocalLength    string     `json:"focalLength"`
	DateTaken      *time.Time `json:"dateTaken"`
	Orientation    int        `json:"orientation"`
	ColorSpace     string     `json:"colorSpace"`
	Software       string     `json:"software"`
	GeolocationRef *uint      `gorm:"index" json:"geolocationRef"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// TableName maps ImageMetadataNew to the image_metadata_new table.
func (ImageMetadataNew) TableName() string { return "image_metadata_new" }

// GeolocationCache mirrors the geolocation_caches table.
type GeolocationCache struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	GPSLatitude  float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLatitude"`
	GPSLongitude float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLongitude"`
	NameLocal    string  `gorm:"type:text" json:"nameLocal"`
	NameEng      string  `gorm:"type:text" json:"nameEng"`
}

// GalleryFolder mirrors the gallery_folders table.
type GalleryFolder struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SupportedExtensions contains all supported image file extensions.
var SupportedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
}

// ---------------------------------------------------------------------------
// Nominatim client
// ---------------------------------------------------------------------------

// NominatimClient provides reverse geocoding via OpenStreetMap Nominatim.
type NominatimClient struct {
	httpClient *http.Client
	baseURL    string
}

// ReverseGeocodeResult holds bilingual location names.
type ReverseGeocodeResult struct {
	NameLocal string
	NameEng   string
}

type nominatimReverseJSON struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		City         string `json:"city"`
		Town         string `json:"town"`
		Village      string `json:"village"`
		State        string `json:"state"`
		Country      string `json:"country"`
		Hamlet       string `json:"hamlet"`
		Municipality string `json:"municipality"`
	} `json:"address"`
}

// NewNominatimClient creates a NominatimClient with sensible defaults.
func NewNominatimClient() *NominatimClient {
	return &NominatimClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    "https://nominatim.openstreetmap.org",
	}
}

// ReverseGeocode performs reverse geocoding for the given coordinates.
func (n *NominatimClient) ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error) {
	url := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json&zoom=10&addressdetails=1&accept-language=en",
		n.baseURL, lat, lng)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create reverse request: %w", err)
	}
	req.Header.Set("User-Agent", "ImageToolkitGeoMigration/1.0")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim reverse request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim reverse returned status %d", resp.StatusCode)
	}

	var raw nominatimReverseJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode nominatim reverse response: %w", err)
	}

	nameLocal := raw.Address.City
	if nameLocal == "" {
		nameLocal = raw.Address.Town
	}
	if nameLocal == "" {
		nameLocal = raw.Address.Village
	}
	if nameLocal == "" {
		nameLocal = raw.Address.Hamlet
	}
	if nameLocal == "" {
		nameLocal = raw.Address.Municipality
	}
	if nameLocal == "" {
		nameLocal = raw.Address.State
	}

	return &ReverseGeocodeResult{
		NameLocal: nameLocal,
		NameEng:   raw.DisplayName,
	}, nil
}

// errRateLimited is returned when Nominatim returns HTTP 429.
var errRateLimited = errors.New("nominatim rate limited (HTTP 429)")

// ---------------------------------------------------------------------------
// Nominatim orchestrator (rate-limited, serialized)
// ---------------------------------------------------------------------------

type resolveRequest struct {
	lat, lng float64
	resultCh chan resolveResult
}

type resolveResult struct {
	entry *GeolocationCache
	err   error
}

// orchestrator runs in a dedicated goroutine, serializing all geolocation
// resolve requests. It ensures at least 1s between Nominatim HTTP calls and
// handles HTTP 429 with exponential backoff.
func orchestrator(db *gorm.DB, nominatim *NominatimClient, ch <-chan resolveRequest, done chan<- struct{}) {
	defer close(done)

	var lastCall time.Time

	for req := range ch {
		entry, err := resolveOne(db, nominatim, req.lat, req.lng, &lastCall)
		req.resultCh <- resolveResult{entry: entry, err: err}
	}
}

// resolveOne handles a single resolve request: cache check, optional Nominatim
// call with rate limiting, and cache insert.
func resolveOne(db *gorm.DB, nominatim *NominatimClient, lat, lng float64, lastCall *time.Time) (*GeolocationCache, error) {
	// 1. Check cache
	var entry GeolocationCache
	err := db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error
	if err == nil {
		return &entry, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("geolocation cache lookup failed: %w", err)
	}

	// 2. Cache miss — rate-limit then call Nominatim
	elapsed := time.Since(*lastCall)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	backoff := 5 * time.Second
	for {
		result, err := nominatim.ReverseGeocode(lat, lng)
		*lastCall = time.Now()
		if err == nil {
			entry = GeolocationCache{
				GPSLatitude:  lat,
				GPSLongitude: lng,
				NameLocal:    result.NameLocal,
				NameEng:      result.NameEng,
			}
			break
		}
		if !errors.Is(err, errRateLimited) {
			return nil, fmt.Errorf("nominatim reverse geocode failed: %w", err)
		}
		// HTTP 429 — exponential backoff
		log.Printf("Nominatim rate limited, backing off %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}

	// 3. Insert into cache (ON CONFLICT DO NOTHING + re-query)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
		// Re-query on conflict
		if err := db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
			return nil, fmt.Errorf("failed to query geolocation cache after conflict: %w", err)
		}
		return &entry, nil
	}

	// If DoNothing was triggered, GORM may not populate entry.ID; re-query
	if entry.ID == 0 {
		if err := db.Where("gps_latitude = ? AND gps_longitude = ?", lat, lng).First(&entry).Error; err != nil {
			return nil, fmt.Errorf("failed to re-query geolocation cache: %w", err)
		}
	}

	return &entry, nil
}

// ---------------------------------------------------------------------------
// ExifTool GPS extraction
// ---------------------------------------------------------------------------

var et *exiftool.Exiftool

// initExifTool checks for exiftool binary and initializes the wrapper.
func initExifTool() error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool binary not found in PATH: %w", err)
	}
	var err error
	et, err = exiftool.NewExiftool()
	if err != nil {
		return fmt.Errorf("failed to initialize exiftool: %w", err)
	}
	return nil
}

// extractGPSCoordinates reads GPS coordinates from an image file's EXIF metadata.
// Returns (lat, lng, true) if GPS data is found, (0, 0, false) otherwise.
func extractGPSCoordinates(filePath string) (float64, float64, bool) {
	if et == nil {
		return 0, 0, false
	}

	fileInfos := et.ExtractMetadata(filePath)
	if len(fileInfos) == 0 || fileInfos[0].Err != nil {
		return 0, 0, false
	}

	fi := fileInfos[0]

	// Method 1: Try direct GPSLatitude/GPSLongitude as float
	if lat, err := fi.GetFloat("GPSLatitude"); err == nil {
		if lng, err := fi.GetFloat("GPSLongitude"); err == nil {
			return lat, lng, true
		}
	}

	// Method 2: Try GPSLatitude/GPSLongitude as string and parse
	if latStr, err := fi.GetString("GPSLatitude"); err == nil {
		if lngStr, err := fi.GetString("GPSLongitude"); err == nil {
			lat, latOk := parseGPSString(latStr)
			lng, lngOk := parseGPSString(lngStr)
			if latOk && lngOk {
				if ref, err := fi.GetString("GPSLatitudeRef"); err == nil && (ref == "S" || ref == "s") {
					lat = -lat
				}
				if ref, err := fi.GetString("GPSLongitudeRef"); err == nil && (ref == "W" || ref == "w") {
					lng = -lng
				}
				return lat, lng, true
			}
		}
	}

	// Method 3: Try GPSPosition if available
	if gpsPos, err := fi.GetString("GPSPosition"); err == nil {
		if lat, lng, ok := parseGPSPosition(gpsPos); ok {
			return lat, lng, true
		}
	}

	return 0, 0, false
}

// parseGPSString parses GPS coordinate strings in various formats.
func parseGPSString(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = strings.TrimRight(s, "NSEWnesw ")

	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val, true
	}

	s = strings.ReplaceAll(s, "deg", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.TrimSpace(s)

	parts := strings.Fields(s)
	if len(parts) >= 2 {
		deg, err1 := strconv.ParseFloat(parts[0], 64)
		min, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			sec := 0.0
			if len(parts) >= 3 {
				sec, _ = strconv.ParseFloat(parts[2], 64)
			}
			return deg + min/60.0 + sec/3600.0, true
		}
	}
	return 0, false
}

// parseGPSPosition tries to parse combined GPSPosition field.
func parseGPSPosition(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}

	parts := strings.Split(s, ",")
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}

	parts = strings.Fields(s)
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lng, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}
	return 0, 0, false
}

// ---------------------------------------------------------------------------
// Config loading
// ---------------------------------------------------------------------------

type dbConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func loadConfig() (*dbConfig, error) {
	envPath := filepath.Join("..", "backend", ".env")
	if err := godotenv.Load(envPath); err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", envPath, err)
	}

	return &dbConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		Name:     getEnv("DB_NAME", "image_toolkit"),
	}, nil
}

func getEnv(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}

// ---------------------------------------------------------------------------
// File collection
// ---------------------------------------------------------------------------

// collectImageFiles walks the given folder paths and returns all image file paths.
func collectImageFiles(folderPaths []string) []string {
	var files []string
	for _, folder := range folderPaths {
		err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				log.Printf("WARN: cannot access %s: %v", path, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if SupportedExtensions[ext] {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			log.Printf("WARN: error walking folder %s: %v", folder, err)
		}
	}
	return files
}

// ---------------------------------------------------------------------------
// Worker pool
// ---------------------------------------------------------------------------

func worker(
	id int,
	db *gorm.DB,
	fileChan <-chan string,
	resolveChan chan<- resolveRequest,
	totalFiles int,
	processed *atomic.Int64,
	skipped *atomic.Int64,
	gpsFound *atomic.Int64,
	updated *atomic.Int64,
	newMeta *atomic.Int64,
	errCount *atomic.Int64,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for filePath := range fileChan {
		// Check if file already has geolocation_ref set (resumability)
		if isAlreadyProcessed(db, filePath) {
			skipped.Add(1)
			p := processed.Add(1)
			if p%50 == 0 || p == int64(totalFiles) {
				logProgress(totalFiles, processed, skipped, gpsFound, updated, newMeta, errCount)
			}
			continue
		}

		lat, lng, hasGPS := extractGPSCoordinates(filePath)

		if hasGPS && lat != 0 && lng != 0 {
			gpsFound.Add(1)

			// Resolve geolocation via orchestrator
			resultCh := make(chan resolveResult, 1)
			resolveChan <- resolveRequest{lat: lat, lng: lng, resultCh: resultCh}
			res := <-resultCh

			if res.err != nil {
				log.Printf("Worker %d: resolve error for %s: %v", id, filepath.Base(filePath), res.err)
				errCount.Add(1)
			} else {
				// Try updating existing image_metadata row
				tx := db.Model(&ImageMetadata{}).
					Where("image_file_id = (SELECT id FROM image_files WHERE path = ?)", filePath).
					Update("geolocation_ref", res.entry.ID)

				if tx.Error != nil {
					log.Printf("Worker %d: update error for %s: %v", id, filepath.Base(filePath), tx.Error)
					errCount.Add(1)
				} else if tx.RowsAffected > 0 {
					updated.Add(1)
				} else {
					// No image_metadata row — look up image_files.id
					var imgFile ImageFile
					if err := db.Where("path = ?", filePath).First(&imgFile).Error; err != nil {
						log.Printf("Worker %d: image_files lookup failed for %s: %v", id, filepath.Base(filePath), err)
						errCount.Add(1)
					} else {
						// Insert into image_metadata_new
						newRow := ImageMetadataNew{
							ImageFileID:    imgFile.ID,
							GeolocationRef: &res.entry.ID,
						}
						if err := db.Create(&newRow).Error; err != nil {
							log.Printf("Worker %d: insert into image_metadata_new failed for %s: %v", id, filepath.Base(filePath), err)
							errCount.Add(1)
						} else {
							newMeta.Add(1)
						}
					}
				}
			}
		}

		// Progress reporting
		p := processed.Add(1)
		if p%50 == 0 || p == int64(totalFiles) {
			logProgress(totalFiles, processed, skipped, gpsFound, updated, newMeta, errCount)
		}
	}
}

// isAlreadyProcessed checks if a file already has geolocation_ref set
// in either image_metadata or image_metadata_new table.
func isAlreadyProcessed(db *gorm.DB, filePath string) bool {
	// Check image_metadata table
	var meta ImageMetadata
	err := db.Select("geolocation_ref").
		Where("image_file_id = (SELECT id FROM image_files WHERE path = ?)", filePath).
		First(&meta).Error
	if err == nil && meta.GeolocationRef != nil {
		return true
	}

	// Check image_metadata_new table
	var metaNew ImageMetadataNew
	err = db.Select("geolocation_ref").
		Where("image_file_id = (SELECT id FROM image_files WHERE path = ?)", filePath).
		First(&metaNew).Error
	if err == nil && metaNew.GeolocationRef != nil {
		return true
	}

	return false
}

// logProgress outputs the current progress status.
func logProgress(totalFiles int, processed, skipped, gpsFound, updated, newMeta, errCount *atomic.Int64) {
	log.Printf("[Progress] %d/%d files | Skipped: %d | GPS: %d | Updated: %d | New metadata: %d | Errors: %d",
		processed.Load(), totalFiles,
		skipped.Load(), gpsFound.Load(), updated.Load(), newMeta.Load(), errCount.Load(),
	)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	startTime := time.Now()
	log.SetFlags(log.Ltime)

	// 1. Load config
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	log.Printf("Loaded config from backend/.env (DB: %s@%s:%s/%s)", cfg.User, cfg.Host, cfg.Port, cfg.Name)

	// 2. Connect to PostgreSQL
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// 3. AutoMigrate image_metadata_new table
	if err := db.AutoMigrate(&ImageMetadataNew{}); err != nil {
		log.Fatalf("Failed to migrate image_metadata_new: %v", err)
	}
	log.Println("Ensured image_metadata_new table exists")

	// 4. Init exiftool
	if err := initExifTool(); err != nil {
		log.Fatalf("ExifTool init failed: %v", err)
	}
	defer et.Close()
	log.Println("ExifTool initialized")

	// 5. Start Nominatim orchestrator
	nominatim := NewNominatimClient()
	resolveChan := make(chan resolveRequest, 64)
	orchDone := make(chan struct{})
	go orchestrator(db, nominatim, resolveChan, orchDone)
	log.Println("Nominatim orchestrator started")

	// 6. Query gallery folders
	var folders []GalleryFolder
	if err := db.Find(&folders).Error; err != nil {
		log.Fatalf("Failed to query gallery_folders: %v", err)
	}
	if len(folders) == 0 {
		log.Fatal("No gallery folders found in database")
	}
	folderPaths := make([]string, len(folders))
	for i, f := range folders {
		folderPaths[i] = f.Path
	}
	log.Printf("Found %d gallery folder(s)", len(folders))

	// 7. Collect all image files
	allFiles := collectImageFiles(folderPaths)
	totalFiles := len(allFiles)
	if totalFiles == 0 {
		log.Fatal("No image files found in gallery folders")
	}
	log.Printf("Found %d image files across %d folder(s)", totalFiles, len(folders))

	// 8. Start workers
	const numWorkers = 2
	fileChan := make(chan string, 64)

	var processed, skipped, gpsFound, updated, newMeta, errCount atomic.Int64
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, db, fileChan, resolveChan, totalFiles,
			&processed, &skipped, &gpsFound, &updated, &newMeta, &errCount, &wg)
	}

	// 9. Feed files to workers
	for _, f := range allFiles {
		fileChan <- f
	}
	close(fileChan)

	// Wait for all workers to finish
	wg.Wait()

	// 10. Close orchestrator and wait
	close(resolveChan)
	<-orchDone

	// 11. Final summary
	elapsed := time.Since(startTime)
	log.Println("========================================")
	log.Printf("Migration complete in %s", elapsed.Round(time.Millisecond))
	log.Printf("  Total files scanned : %d", processed.Load())
	log.Printf("  Skipped (already done): %d", skipped.Load())
	log.Printf("  GPS found           : %d", gpsFound.Load())
	log.Printf("  Updated (existing)  : %d", updated.Load())
	log.Printf("  New metadata rows   : %d", newMeta.Load())
	log.Printf("  Errors              : %d", errCount.Load())
	log.Println("========================================")
}
