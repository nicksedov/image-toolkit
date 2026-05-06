package imaging

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/disintegration/imaging"
	"github.com/barasher/go-exiftool"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"image-toolkit/internal/domain"
)

// exifTool is the global exiftool instance
var exifTool *exiftool.Exiftool

// InitExifTool initializes the exiftool library
func InitExifTool() error {
	var err error
	exifTool, err = exiftool.NewExiftool()
	if err != nil {
		return fmt.Errorf("failed to initialize exiftool: %w", err)
	}
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

	// Attempt EXIF extraction using exiftool
	extractExifWithExiftool(filePath, meta)

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

// extractExifWithExiftool extracts metadata using the exiftool library.
// This library handles modern smartphone formats including Huawei P30.
func extractExifWithExiftool(filePath string, meta *domain.ImageMetadata) {
	if exifTool == nil {
		log.Printf("WARNING: exiftool not initialized, skipping EXIF extraction for %s", filepath.Base(filePath))
		return
	}

	// Extract metadata from file
	fileMeta := exifTool.ExtractMetadata(filePath)
	if len(fileMeta) == 0 {
		log.Printf("WARNING: No metadata extracted for %s", filepath.Base(filePath))
		return
	}

	exifData := fileMeta[0]
	if exifData.Err != nil {
		log.Printf("WARNING: Error extracting metadata for %s: %v", filepath.Base(filePath), exifData.Err)
		return
	}

	// Camera model
	if model, err := exifData.GetString("Model"); err == nil && model != "" {
		meta.CameraModel = strings.TrimSpace(model)
	}

	// Lens model
	if lens, err := exifData.GetString("LensModel"); err == nil && lens != "" {
		meta.LensModel = strings.TrimSpace(lens)
	}

	// ISO
	if iso, err := exifData.GetInt("ISO"); err == nil && iso > 0 {
		meta.ISO = int(iso)
	}

	// Aperture (FNumber)
	if aperture, err := exifData.GetFloat("Aperture"); err == nil && aperture > 0 {
		meta.Aperture = fmt.Sprintf("f/%.1f", aperture)
	}

	// Shutter speed (ExposureTime)
	if shutterSpeed, err := exifData.GetFloat("ExposureTime"); err == nil && shutterSpeed > 0 {
		if shutterSpeed >= 1 {
			meta.ShutterSpeed = fmt.Sprintf("%.0fs", shutterSpeed)
		} else {
			// Convert to fraction like 1/250
			denom := int(1.0 / shutterSpeed)
			meta.ShutterSpeed = fmt.Sprintf("1/%ds", denom)
		}
	}

	// Focal length
	if focalLength, err := exifData.GetFloat("FocalLength"); err == nil && focalLength > 0 {
		if focalLength == math.Trunc(focalLength) {
			meta.FocalLength = fmt.Sprintf("%.0fmm", focalLength)
		} else {
			meta.FocalLength = fmt.Sprintf("%.1fmm", focalLength)
		}
	}

	// Date taken - try multiple possible tags
	dateTaken := extractDateTaken(exifData)
	if dateTaken != nil {
		meta.DateTaken = dateTaken
	}

	// Orientation
	if orientation, err := exifData.GetInt("Orientation"); err == nil && orientation > 0 {
		meta.Orientation = int(orientation)
	}

	// Color space
	if colorSpace, err := exifData.GetString("ColorSpace"); err == nil && colorSpace != "" {
		meta.ColorSpace = colorSpace
	} else if colorSpaceInt, err := exifData.GetInt("ColorSpace"); err == nil {
		meta.ColorSpace = colorSpaceName(int(colorSpaceInt))
	}

	// Software
	if software, err := exifData.GetString("Software"); err == nil && software != "" {
		meta.Software = strings.TrimSpace(software)
	}

	// GPS coordinates - this is where exiftool excels with Huawei/modern phones
	extractGPS(exifData, meta)
}

// extractDateTaken tries multiple EXIF date fields to get the date taken
func extractDateTaken(exifData exiftool.FileMetadata) *time.Time {
	// Try DateTimeOriginal first (most common)
	if dateTime, err := exifData.GetString("DateTimeOriginal"); err == nil && dateTime != "" {
		if t, err := parseExifDate(dateTime); err == nil {
			return &t
		}
	}

	// Try CreateDate as fallback
	if dateTime, err := exifData.GetString("CreateDate"); err == nil && dateTime != "" {
		if t, err := parseExifDate(dateTime); err == nil {
			return &t
		}
	}

	// Try DateTimeDigitized
	if dateTime, err := exifData.GetString("DateTimeDigitized"); err == nil && dateTime != "" {
		if t, err := parseExifDate(dateTime); err == nil {
			return &t
		}
	}

	return nil
}

// parseExifDate parses EXIF date format "YYYY:MM:DD HH:MM:SS"
func parseExifDate(dateStr string) (time.Time, error) {
	// EXIF format: "2024:01:15 14:30:45"
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) < 19 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
	}
	
	// Replace colon with dash for date part
	dateStr = dateStr[:4] + "-" + dateStr[5:7] + "-" + dateStr[8:]
	
	return time.Parse("2006-01-02 15:04:05", dateStr)
}

// extractGPS extracts GPS coordinates from EXIF data
func extractGPS(exifData exiftool.FileMetadata, meta *domain.ImageMetadata) {
	// Try GPS latitude and longitude
	latStr, latErr := exifData.GetString("GPSLatitude")
	lngStr, lngErr := exifData.GetString("GPSLongitude")
	
	if latErr == nil && lngErr == nil && latStr != "" && lngStr != "" {
		lat, latConvErr := convertGPSCoordinate(latStr)
		lng, lngConvErr := convertGPSCoordinate(lngStr)
		
		if latConvErr == nil && lngConvErr == nil {
			// Handle GPS reference (N/S for latitude, E/W for longitude)
			if latRef, err := exifData.GetString("GPSLatitudeRef"); err == nil && strings.ToUpper(latRef) == "S" {
				lat = -lat
			}
			if lngRef, err := exifData.GetString("GPSLongitudeRef"); err == nil && strings.ToUpper(lngRef) == "W" {
				lng = -lng
			}
			
			meta.GPSLatitude = &lat
			meta.GPSLongitude = &lng
			return
		}
	}
	
	// Fallback: try composite GPSLatitude/GPSLongitude (decimal degrees)
	if lat, err := exifData.GetFloat("GPSLatitude"); err == nil {
		if lng, err := exifData.GetFloat("GPSLongitude"); err == nil {
			meta.GPSLatitude = &lat
			meta.GPSLongitude = &lng
		}
	}
}

// convertGPSCoordinate converts GPS coordinate string to decimal degrees
// Format can be: "52 deg 22' 11.12"" or "52.369756" or "52, 22, 11.12"
func convertGPSCoordinate(coordStr string) (float64, error) {
	coordStr = strings.TrimSpace(coordStr)
	
	// Try parsing as decimal first
	if val, err := strconv.ParseFloat(coordStr, 64); err == nil {
		return val, nil
	}
	
	// Try parsing DMS format: "52 deg 22' 11.12""
	// Remove degree, minute, second symbols
	coordStr = strings.ReplaceAll(coordStr, "deg", ",")
	coordStr = strings.ReplaceAll(coordStr, "'", ",")
	coordStr = strings.ReplaceAll(coordStr, "\"", "")
	coordStr = strings.ReplaceAll(coordStr, "，", ",") // Handle Chinese comma
	
	parts := strings.Split(coordStr, ",")
	if len(parts) >= 2 {
		degrees, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return 0, err
		}
		
		minutes, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			minutes = 0
		}
		
		seconds := 0.0
		if len(parts) >= 3 {
			seconds, err = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
			if err != nil {
				seconds = 0
			}
		}
		
		return degrees + minutes/60 + seconds/3600, nil
	}
	
	return 0, fmt.Errorf("unable to parse GPS coordinate: %s", coordStr)
}

// formatExposureTime formats a rational exposure time value.
func formatExposureTime(num, denom int64) string {
	if num == 0 {
		return "0s"
	}
	if num >= denom {
		val := float64(num) / float64(denom)
		if val == math.Trunc(val) {
			return fmt.Sprintf("%.0fs", val)
		}
		return fmt.Sprintf("%.1fs", val)
	}
	// Show as fraction, e.g. "1/250s"
	simplified := denom / num
	return fmt.Sprintf("1/%ds", simplified)
}

// colorSpaceName maps EXIF ColorSpace integer to a human-readable name.
func colorSpaceName(v int) string {
	switch v {
	case 1:
		return "sRGB"
	case 65535:
		return "Uncalibrated"
	default:
		return fmt.Sprintf("Unknown (%d)", v)
	}
}

// HasExifData returns true if any meaningful EXIF field is populated.
func HasExifData(meta *domain.ImageMetadata) bool {
	return meta.CameraModel != "" || meta.LensModel != "" || meta.ISO != 0 ||
		meta.Aperture != "" || meta.ShutterSpeed != "" || meta.FocalLength != "" ||
		meta.DateTaken != nil || meta.Software != "" || meta.GPSLatitude != nil
}
