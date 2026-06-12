package imaging

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
	_ "github.com/disintegration/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"image-toolkit/internal/domain"
)

// trashTimestampFormat is used for backup file naming.
const trashTimestampFormat = "20060102_150405"

// exifTool is the global exiftool instance
var exifTool *exiftool.Exiftool

// InitExifTool initializes the global exiftool instance.
// It checks for exiftool binary availability and creates the Exiftool wrapper.
// Returns an error if exiftool is not found or cannot be initialized.
func InitExifTool() error {
	// Check if exiftool binary is available
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool binary not found in PATH: %w", err)
	}

	et, err := exiftool.NewExiftool()
	if err != nil {
		return fmt.Errorf("failed to initialize exiftool: %w", err)
	}

	exifTool = et
	log.Printf("EXIF: go-exiftool initialized successfully")
	return nil
}

// extractMetadata reads EXIF metadata and image dimensions from a file.
// It always attempts to get dimensions; EXIF fields are best-effort.
func extractMetadata(filePath string) (*domain.ImageMetadata, error) {
	meta := &domain.ImageMetadata{}

	// Get image dimensions (works for all supported formats)
	if w, h, err := getImageDimensions(filePath); err == nil {
		meta.Width = w
		meta.Height = h
	}

	// Attempt EXIF extraction
	if exifTool != nil {
		extractExifFields(filePath, meta)
	} else {
		log.Printf("EXIF: exiftool not initialized, skipping EXIF extraction for %s", filepath.Base(filePath))
	}

	return meta, nil
}

// getImageDimensions reads only the image header to get width and height.
func getImageDimensions(filePath string) (int, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// extractExifFields attempts to read all EXIF fields from the file using go-exiftool.
// Each field is extracted independently; failures are logged.
func extractExifFields(filePath string, meta *domain.ImageMetadata) {
	fileInfos := exifTool.ExtractMetadata(filePath)
	if len(fileInfos) == 0 {
		log.Printf("EXIF: No metadata returned for %s", filepath.Base(filePath))
		return
	}

	fi := fileInfos[0]
	if fi.Err != nil {
		log.Printf("EXIF: Error extracting metadata from %s: %v", filepath.Base(filePath), fi.Err)
		return
	}

	baseName := filepath.Base(filePath)

	// Camera model
	if model, err := fi.GetString("Model"); err == nil && model != "" {
		meta.CameraModel = cleanString(model)
		log.Printf("EXIF %s: CameraModel=%s", baseName, meta.CameraModel)
	}

	// Lens model
	if lens, err := fi.GetString("LensModel"); err == nil && lens != "" {
		meta.LensModel = cleanString(lens)
	}

	// ISO
	if iso, err := fi.GetInt("ISO"); err == nil {
		meta.ISO = int(iso)
	}

	// Aperture (FNumber)
	if aperture, err := fi.GetFloat("FNumber"); err == nil {
		meta.Aperture = fmt.Sprintf("f/%.1f", aperture)
	}

	// Shutter speed (ExposureTime)
	if exposureTime, err := fi.GetFloat("ExposureTime"); err == nil {
		meta.ShutterSpeed = formatExposureTimeFloat(exposureTime)
	}

	// Focal length
	if focalLength, err := fi.GetFloat("FocalLength"); err == nil {
		if focalLength == math.Trunc(focalLength) {
			meta.FocalLength = fmt.Sprintf("%.0fmm", focalLength)
		} else {
			meta.FocalLength = fmt.Sprintf("%.1fmm", focalLength)
		}
	}

	// Date taken - try multiple fields
	extractDateTaken(fi, meta, baseName)

	// Orientation
	if orientation, err := fi.GetString("Orientation"); err == nil && orientation != "" {
		meta.Orientation = parseOrientation(orientation)
	}

	// Color space
	if colorSpace, err := fi.GetString("ColorSpace"); err == nil && colorSpace != "" {
		meta.ColorSpace = parseColorSpace(colorSpace)
	}

	// Software
	if software, err := fi.GetString("Software"); err == nil && software != "" {
		meta.Software = cleanString(software)
	}

	// GPS coordinates - with multiple fallback methods
	// Note: GPS is no longer stored on ImageMetadata directly;
	// it's resolved via GeolocationService -> GeolocationCache.
	// extractGPSCoordinates() provides standalone GPS extraction.
}

// extractDateTaken tries multiple EXIF date fields to populate DateTaken.
func extractDateTaken(fi exiftool.FileMetadata, meta *domain.ImageMetadata, baseName string) {
	// Try DateTimeOriginal first (most common for photos)
	dateFields := []string{"DateTimeOriginal", "CreateDate", "ModifyDate", "DateTime"}

	for _, field := range dateFields {
		if dateStr, err := fi.GetString(field); err == nil && dateStr != "" {
			if t, err := parseExifDate(dateStr); err == nil {
				meta.DateTaken = &t
				log.Printf("EXIF %s: DateTaken=%s (from %s)", baseName, t.Format("2006-01-02 15:04:05"), field)
				return
			}
		}
	}
}

// extractGPS extracts GPS coordinates with multiple fallback methods.
// Deprecated: GPS fields removed from domain.ImageMetadata.
// Use extractGPSCoordinates() instead.

// extractGPSCoordinates reads GPS coordinates from an image file's EXIF metadata.
// Returns (lat, lng, true) if GPS data is found, (0, 0, false) otherwise.
func extractGPSCoordinates(filePath string) (float64, float64, bool) {
	if exifTool == nil {
		return 0, 0, false
	}

	fileInfos := exifTool.ExtractMetadata(filePath)
	if len(fileInfos) == 0 || fileInfos[0].Err != nil {
		return 0, 0, false
	}

	fi := fileInfos[0]
	baseName := filepath.Base(filePath)

	// Method 1: Try direct GPSLatitude/GPSLongitude as float
	if lat, err := fi.GetFloat("GPSLatitude"); err == nil {
		if lng, err := fi.GetFloat("GPSLongitude"); err == nil {
			log.Printf("EXIF %s: GPS via float: lat=%.8f, lng=%.8f", baseName, lat, lng)
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
				log.Printf("EXIF %s: GPS via string parse: lat=%.8f, lng=%.8f", baseName, lat, lng)
				return lat, lng, true
			}
		}
	}

	// Method 3: Try GPSPosition if available
	if gpsPos, err := fi.GetString("GPSPosition"); err == nil {
		if lat, lng, ok := parseGPSPosition(gpsPos); ok {
			log.Printf("EXIF %s: GPS via GPSPosition: lat=%.8f, lng=%.8f", baseName, lat, lng)
			return lat, lng, true
		}
	}

	return 0, 0, false
}

// parseGPSString parses GPS coordinate strings in various formats:
// "41 deg 24' 12.2" N", "41.40338", "41 24 12.2", etc.
func parseGPSString(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	// Remove hemisphere reference if present
	s = strings.TrimRight(s, "NSEWnesw ")

	// Try parsing as a simple float first
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val, true
	}

	// Try parsing "deg min sec" or "deg min'sec\"" format
	// Remove common symbols
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
// Format is typically "41.40338, 12.13278" or similar.
func parseGPSPosition(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}

	// Try comma-separated
	parts := strings.Split(s, ",")
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}

	// Try space-separated
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

// parseOrientation converts orientation string to integer value.
func parseOrientation(s string) int {
	s = strings.TrimSpace(s)

	// Try direct integer
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}

	// Parse text orientations
	lower := strings.ToLower(s)
	if strings.Contains(lower, "rotate") {
		if strings.Contains(lower, "90") {
			if strings.Contains(lower, "cw") || strings.Contains(lower, "normal") {
				return 6 // 90 CW
			}
			return 8 // 90 CCW (or 270 CW)
		}
		if strings.Contains(lower, "180") {
			return 3
		}
	}

	if strings.Contains(lower, "mirror") || strings.Contains(lower, "flip") {
		if strings.Contains(lower, "horizontal") {
			return 2
		}
		if strings.Contains(lower, "vertical") {
			return 4
		}
	}

	if strings.Contains(lower, "normal") || strings.Contains(lower, "horizontal") {
		return 1
	}

	return 0
}

// parseColorSpace converts color space string to a standardized name.
func parseColorSpace(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)

	switch {
	case lower == "srgb", strings.Contains(lower, "srgb"):
		return "sRGB"
	case lower == "adobe rgb", strings.Contains(lower, "adobe"):
		return "Adobe RGB"
	case strings.Contains(lower, "uncalibrat"):
		return "Uncalibrated"
	default:
		return s
	}
}

// parseExifDate parses EXIF date format "YYYY:MM:DD HH:MM:SS"
func parseExifDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) < 19 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
	}

	// EXIF format: "2024:01:15 14:30:45"
	// Convert to: "2024-01-15 14:30:45"
	dateStr = dateStr[:4] + "-" + dateStr[5:7] + "-" + dateStr[8:]

	return time.Parse("2006-01-02 15:04:05", dateStr)
}

// cleanString removes trailing whitespace and null characters from strings
func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "\x00")
	return s
}

// formatExposureTimeFloat formats a float exposure time value.
func formatExposureTimeFloat(val float64) string {
	if val <= 0 {
		return "0s"
	}
	if val >= 1 {
		if val == math.Trunc(val) {
			return fmt.Sprintf("%.0fs", val)
		}
		return fmt.Sprintf("%.1fs", val)
	}
	// Show as fraction, e.g. "1/250s"
	denom := int(1.0 / val)
	return fmt.Sprintf("1/%ds", denom)
}

// HasExifData returns true if any meaningful EXIF field is populated.
func HasExifData(meta *domain.ImageMetadata) bool {
	return meta.CameraModel != "" || meta.LensModel != "" || meta.ISO != 0 ||
		meta.Aperture != "" || meta.ShutterSpeed != "" || meta.FocalLength != "" ||
		meta.DateTaken != nil || meta.Software != "" || meta.GeolocationRef != nil
}

// WriteGPS backs up the original file and writes GPS coordinates to its EXIF metadata.
// If trashDir is non-empty, the backup is created there; otherwise alongside the original file.
func WriteGPS(filePath, trashDir string, lat, lng float64) error {
	// Validate coordinates
	if lat < -90 || lat > 90 {
		return fmt.Errorf("invalid latitude: %f (must be between -90 and 90)", lat)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("invalid longitude: %f (must be between -180 and 180)", lng)
	}

	// Check JPEG extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".jpg" && ext != ".jpeg" {
		return fmt.Errorf("GPS can only be written to JPEG files, got: %s", ext)
	}

	// Create backup before modifying the file
	if err := createBackup(filePath, trashDir); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Determine hemisphere references and absolute values for exiftool
	latRef := "N"
	lngRef := "E"
	absLat := lat
	absLng := lng
	if lat < 0 {
		latRef = "S"
		absLat = -lat
	}
	if lng < 0 {
		lngRef = "W"
		absLng = -lng
	}

	// Write GPS using exiftool CLI
	cmd := exec.Command("exiftool",
		"-overwrite_original",
		fmt.Sprintf("-GPSLatitude=%.8f", absLat),
		fmt.Sprintf("-GPSLatitudeRef=%s", latRef),
		fmt.Sprintf("-GPSLongitude=%.8f", absLng),
		fmt.Sprintf("-GPSLongitudeRef=%s", lngRef),
		filePath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("EXIF WriteGPS: exiftool failed for %s: %v, output: %s", filepath.Base(filePath), err, string(output))
		return fmt.Errorf("exiftool failed: %w", err)
	}

	log.Printf("EXIF WriteGPS: GPS written to %s (lat=%.8f, lng=%.8f)", filepath.Base(filePath), lat, lng)
	return nil
}

// createBackup copies the original file to a backup location before EXIF modification.
func createBackup(filePath, trashDir string) error {
	dir := filepath.Dir(filePath)
	if trashDir != "" {
		dir = trashDir
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create trash directory: %w", err)
		}
	}

	ext := filepath.Ext(filePath)
	nameWithoutExt := strings.TrimSuffix(filepath.Base(filePath), ext)
	backupName := fmt.Sprintf("%s_backup_%s%s", nameWithoutExt, time.Now().Format(trashTimestampFormat), ext)
	backupPath := filepath.Join(dir, backupName)

	src, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	log.Printf("EXIF WriteGPS: backup created at %s", backupPath)
	return nil
}
