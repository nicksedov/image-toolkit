package llm

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"os"
	"strings"

	"github.com/disintegration/imaging"
)

// resizeImageForLLM reads an image from the given path and downsizes it if its
// pixel count exceeds maxMegapixels. The returned bytes are JPEG-encoded and
// the media type is inferred from the file extension.
func resizeImageForLLM(imagePath string, maxMegapixels float64) ([]byte, string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width > 0 && height > 0 && maxMegapixels > 0 {
		megapixels := float64(width*height) / 1_000_000.0
		if megapixels > maxMegapixels {
			scale := math.Sqrt(maxMegapixels * 1_000_000.0 / float64(width*height))
			newWidth := int(math.Round(float64(width) * scale))
			newHeight := int(math.Round(float64(height) * scale))
			if newWidth > 0 && newHeight > 0 {
				img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
			}
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("failed to encode image: %w", err)
	}

	mediaType := "image/jpeg"
	ext := strings.ToLower(imagePath)
	switch {
	case strings.HasSuffix(ext, ".png"):
		mediaType = "image/png"
	case strings.HasSuffix(ext, ".gif"):
		mediaType = "image/gif"
	case strings.HasSuffix(ext, ".webp"):
		mediaType = "image/webp"
	case strings.HasSuffix(ext, ".tiff") || strings.HasSuffix(ext, ".tif"):
		mediaType = "image/tiff"
	}

	return buf.Bytes(), mediaType, nil
}
