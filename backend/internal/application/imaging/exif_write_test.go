package imaging

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// createTempJPEG creates a minimal valid JPEG file for testing.
func createTempJPEG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	// Minimal valid JPEG: SOI + APP0 + minimal data + EOI
	// This is a 1x1 pixel white JPEG
	data := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08,
		0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3,
		0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6,
		0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
		0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
		0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4,
		0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01,
		0x00, 0x00, 0x3F, 0x00, 0x7B, 0x94, 0x11, 0x00, 0x00, 0x00, 0x00, 0xFF,
		0xD9,
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to create temp JPEG: %v", err)
	}
	return path
}

func TestWriteGPS_InvalidCoordinates(t *testing.T) {
	// Latitude out of range
	err := WriteGPS("/tmp/test.jpg", "", 100, 50)
	if err == nil {
		t.Fatal("expected error for invalid latitude, got nil")
	}
	if !strings.Contains(err.Error(), "latitude") {
		t.Fatalf("expected latitude error, got: %v", err)
	}

	// Longitude out of range
	err = WriteGPS("/tmp/test.jpg", "", 50, 200)
	if err == nil {
		t.Fatal("expected error for invalid longitude, got nil")
	}
	if !strings.Contains(err.Error(), "longitude") {
		t.Fatalf("expected longitude error, got: %v", err)
	}
}

func TestWriteGPS_NotJpeg(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(pngPath, []byte{0x89, 0x50, 0x4E, 0x47}, 0644); err != nil {
		t.Fatal(err)
	}

	err := WriteGPS(pngPath, "", 40.0, -74.0)
	if err == nil {
		t.Fatal("expected error for non-JPEG file, got nil")
	}
	if !strings.Contains(err.Error(), "JPEG") {
		t.Fatalf("expected JPEG error, got: %v", err)
	}

	// Verify no backup was created
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) != 1 {
		t.Fatalf("expected only 1 file (no backup), got %d", len(entries))
	}
}

func TestWriteGPS_FileNotFound(t *testing.T) {
	err := WriteGPS("/nonexistent/path/test.jpg", "", 40.0, -74.0)
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestWriteGPS_BackupCreated(t *testing.T) {
	tmpDir := t.TempDir()
	trashDir := t.TempDir()

	jpegPath := createTempJPEG(t, tmpDir, "photo.jpg")
	origInfo, _ := os.Stat(jpegPath)

	err := WriteGPS(jpegPath, trashDir, 48.8566, 2.3522)
	// exiftool might not be available in CI, so check backup first
	entries, readErr := os.ReadDir(trashDir)
	if readErr != nil {
		t.Fatalf("failed to read trash dir: %v", readErr)
	}

	// Verify backup exists
	found := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "_backup_") && strings.HasSuffix(entry.Name(), ".jpg") {
			found = true
			backupInfo, _ := os.Stat(filepath.Join(trashDir, entry.Name()))
			if backupInfo.Size() != origInfo.Size() {
				t.Errorf("backup size %d != original size %d", backupInfo.Size(), origInfo.Size())
			}
			break
		}
	}
	if !found {
		t.Fatal("backup file not found in trash directory")
	}

	// If exiftool is not available, that's expected
	if err != nil {
		if _, lookErr := exec.LookPath("exiftool"); lookErr != nil {
			t.Skip("exiftool not available, skipping EXIF write verification")
		}
		t.Fatalf("WriteGPS failed: %v", err)
	}
}

func TestWriteGPS_BackupInSameDirWhenNoTrash(t *testing.T) {
	tmpDir := t.TempDir()
	jpegPath := createTempJPEG(t, tmpDir, "photo.jpg")

	_ = WriteGPS(jpegPath, "", 48.8566, 2.3522)

	// Check that backup was created in the same directory
	entries, _ := os.ReadDir(tmpDir)
	found := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "_backup_") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("backup file not found in same directory when trashDir is empty")
	}
}

func TestWriteGPS_Success(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not available in PATH")
	}

	tmpDir := t.TempDir()
	trashDir := t.TempDir()
	jpegPath := createTempJPEG(t, tmpDir, "photo.jpg")

	err := WriteGPS(jpegPath, trashDir, 48.8566, 2.3522)
	if err != nil {
		t.Fatalf("WriteGPS failed: %v", err)
	}

	// Verify GPS was written using exiftool
	cmd := exec.Command("exiftool", "-GPSLatitude", "-GPSLatitudeRef", "-GPSLongitude", "-GPSLongitudeRef", jpegPath)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("exiftool read failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "North") {
		t.Errorf("expected North latitude ref, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "East") {
		t.Errorf("expected East longitude ref, got: %s", outputStr)
	}
}

func TestWriteGPS_NegativeCoordinates(t *testing.T) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not available in PATH")
	}

	tmpDir := t.TempDir()
	trashDir := t.TempDir()
	jpegPath := createTempJPEG(t, tmpDir, "photo.jpg")

	// Sydney, Australia: -33.8688, 151.2093
	err := WriteGPS(jpegPath, trashDir, -33.8688, 151.2093)
	if err != nil {
		t.Fatalf("WriteGPS failed: %v", err)
	}

	// Verify hemisphere handling
	cmd := exec.Command("exiftool", "-GPSLatitude", "-GPSLatitudeRef", "-GPSLongitude", "-GPSLongitudeRef", jpegPath)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("exiftool read failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "South") {
		t.Errorf("expected South latitude ref for negative lat, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "East") {
		t.Errorf("expected East longitude ref for positive lng, got: %s", outputStr)
	}
}
