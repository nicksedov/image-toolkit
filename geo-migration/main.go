package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Domain models (local copies — standalone app does not import main backend)
// ---------------------------------------------------------------------------

// GeolocationCache mirrors the geolocation_caches table.
type GeolocationCache struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	GPSLatitude  float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLatitude"`
	GPSLongitude float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLongitude"`
	NameLocal    string  `gorm:"type:text" json:"nameLocal"`
	NameEng      string  `gorm:"type:text" json:"nameEng"`
}

// ---------------------------------------------------------------------------
// Nominatim client
// ---------------------------------------------------------------------------

// NominatimClient provides reverse geocoding via OpenStreetMap Nominatim.
type NominatimClient struct {
	httpClient *http.Client
	baseURL    string
}

type nominatimReverseJSON struct {
	DisplayName string `json:"display_name"`
}

// NewNominatimClient creates a NominatimClient with sensible defaults.
func NewNominatimClient() *NominatimClient {
	return &NominatimClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    "https://nominatim.openstreetmap.org",
	}
}

// ReverseDisplayName calls Nominatim /reverse and returns the display_name.
func (n *NominatimClient) ReverseDisplayName(lat, lng float64) (string, error) {
	url := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json&zoom=10&addressdetails=1&accept-language=en",
		n.baseURL, lat, lng)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create reverse request: %w", err)
	}
	req.Header.Set("User-Agent", "ImageToolkitGeoMigration/2.0")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("nominatim reverse request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", errRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nominatim reverse returned status %d", resp.StatusCode)
	}

	var raw nominatimReverseJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("failed to decode nominatim reverse response: %w", err)
	}

	return raw.DisplayName, nil
}

// errRateLimited is returned when Nominatim returns HTTP 429.
var errRateLimited = errors.New("nominatim rate limited (HTTP 429)")

// reverseWithRetry calls Nominatim /reverse with rate limiting and exponential backoff.
func reverseWithRetry(nominatim *NominatimClient, lat, lng float64, lastCall *time.Time) (string, error) {
	// Enforce minimum 1s between calls
	elapsed := time.Since(*lastCall)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	backoff := 5 * time.Second
	for {
		displayName, err := nominatim.ReverseDisplayName(lat, lng)
		*lastCall = time.Now()
		if err == nil {
			return displayName, nil
		}
		if !errors.Is(err, errRateLimited) {
			return "", err
		}
		log.Printf("Nominatim rate limited, backing off %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
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
	envPath := filepath.Join(".", ".env")
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
// Group key for batching
// ---------------------------------------------------------------------------

type groupKey struct {
	NameLocal string
	NameEng   string
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
	log.Printf("Loaded config (DB: %s@%s:%s/%s)", cfg.User, cfg.Host, cfg.Port, cfg.Name)

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

	// 3. Load all geolocation_caches records
	var allRecords []GeolocationCache
	if err := db.Find(&allRecords).Error; err != nil {
		log.Fatalf("Failed to load geolocation_caches: %v", err)
	}
	totalRecords := len(allRecords)
	log.Printf("Loaded %d geolocation_cache records", totalRecords)

	if totalRecords == 0 {
		log.Println("Nothing to process — table is empty")
		return
	}

	// 4. Group records by (name_local, name_eng) pair
	groups := make(map[groupKey][]uint)
	for _, r := range allRecords {
		key := groupKey{NameLocal: r.NameLocal, NameEng: r.NameEng}
		groups[key] = append(groups[key], r.ID)
	}
	log.Printf("Found %d unique (name_local, name_eng) groups", len(groups))

	// 5. Build a lookup: first record's coordinates per group for the API call
	coordsByKey := make(map[groupKey]struct{ lat, lng float64 })
	for _, r := range allRecords {
		key := groupKey{NameLocal: r.NameLocal, NameEng: r.NameEng}
		if _, exists := coordsByKey[key]; !exists {
			coordsByKey[key] = struct{ lat, lng float64 }{r.GPSLatitude, r.GPSLongitude}
		}
	}

	// 6. Process each group: call Nominatim /reverse, batch-update name_local
	nominatim := NewNominatimClient()
	var lastCall time.Time

	// Track already-processed record IDs
	processedIDs := make(map[uint]bool)

	var totalUpdated int64
	var totalGroups int
	var totalErrors int
	groupNum := 0

	for key, ids := range groups {
		groupNum++

		// Skip if all IDs in this group were already processed
		allDone := true
		for _, id := range ids {
			if !processedIDs[id] {
				allDone = false
				break
			}
		}
		if allDone {
			log.Printf("[%d/%d] Group %q / %q — already processed, skipping", groupNum, len(groups), key.NameLocal, key.NameEng)
			continue
		}

		coords := coordsByKey[key]
		log.Printf("[%d/%d] Group %q / %q (%d records) — querying Nominatim at (%f, %f)...",
			groupNum, len(groups), key.NameLocal, key.NameEng, len(ids), coords.lat, coords.lng)

		// Call Nominatim /reverse to get the correct display_name
		displayName, err := reverseWithRetry(nominatim, coords.lat, coords.lng, &lastCall)
		if err != nil {
			log.Printf("  ERROR: Nominatim call failed: %v", err)
			totalErrors++
			continue
		}

		// Skip if display_name is empty or unchanged
		if displayName == "" {
			log.Printf("  WARN: empty display_name, skipping")
			totalErrors++
			continue
		}
		if displayName == key.NameLocal {
			log.Printf("  OK: name_local already correct (%q), marking as done", displayName)
			for _, id := range ids {
				processedIDs[id] = true
			}
			totalGroups++
			continue
		}

		// Batch update all records in this group
		result := db.Model(&GeolocationCache{}).
			Where("id IN ?", ids).
			Update("name_local", displayName)
		if result.Error != nil {
			log.Printf("  ERROR: batch update failed: %v", result.Error)
			totalErrors++
			continue
		}

		// Mark all IDs as processed
		for _, id := range ids {
			processedIDs[id] = true
		}

		totalUpdated += result.RowsAffected
		totalGroups++
		log.Printf("  OK: updated %d records: %q → %q", result.RowsAffected, key.NameLocal, displayName)
	}

	// 7. Final summary
	elapsed := time.Since(startTime)
	log.Println("========================================")
	log.Printf("Migration complete in %s", elapsed.Round(time.Millisecond))
	log.Printf("  Total records     : %d", totalRecords)
	log.Printf("  Unique groups     : %d", len(groups))
	log.Printf("  Groups processed  : %d", totalGroups)
	log.Printf("  Records updated   : %d", totalUpdated)
	log.Printf("  Errors            : %d", totalErrors)
	log.Printf("  IDs tracked       : %d", len(processedIDs))
	log.Println("========================================")
}
