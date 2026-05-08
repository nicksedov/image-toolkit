package imaging

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/ocr"
)

// OcrResult represents the result of processing a single image
type OcrResult struct {
	Image          domain.ImageFile
	Classification *domain.OcrClassification
	Boxes          []domain.OcrBoundingBox
	Err            error
}

// processSingleImage performs OCR classification on a single image file
func (om *OcrManager) processSingleImage(image domain.ImageFile, stopRequested func() bool) OcrResult {
	result := OcrResult{Image: image}

	// Check for stop before processing
	if stopRequested() {
		log.Printf("[OCR] Image ID=%d STOPPED before processing", image.ID)
		result.Err = fmt.Errorf("classification stopped")
		return result
	}

	// Open image file
	log.Printf("[OCR] Opening file: %s (image ID=%d)", image.Path, image.ID)
	file, err := os.Open(image.Path)
	if err != nil {
		log.Printf("[OCR] ERROR opening file %s (ID=%d): %v", image.Path, image.ID, err)
		result.Err = fmt.Errorf("failed to open file: %w", err)
		return result
	}
	defer file.Close()
	log.Printf("[OCR] File opened OK: %s (ID=%d)", image.Path, image.ID)

	// Determine content type based on extension
	contentType := om.detectContentType(image.Path)

	// Call OCR API
	log.Printf("[OCR] Calling OCR API for image ID=%d, path=%s, contentType=%s", image.ID, image.Path, contentType)
	ocrStart := time.Now()
	ctx := context.Background()
	ocrResp, err := om.ocrClient.Classify(ctx, file, contentType, ocr.DefaultClassifyParams())
	ocrDuration := time.Since(ocrStart)

	if err != nil {
		log.Printf("[OCR] OCR API FAILED for image ID=%d after %v: %v", image.ID, ocrDuration, err)
		result.Err = fmt.Errorf("OCR classification failed: %w", err)
		return result
	}
	log.Printf("[OCR] OCR API OK for image ID=%d in %v: isText=%v, confidence=%.3f", image.ID, ocrDuration, ocrResp.IsTextDocument, ocrResp.MeanConfidence)

	// Build classification record
	result.Classification = &domain.OcrClassification{
		ImageFileID:        image.ID,
		IsTextDocument:     ocrResp.IsTextDocument,
		MeanConfidence:     ocrResp.MeanConfidence,
		WeightedConfidence: ocrResp.WeightedConfidence,
		TokenCount:         ocrResp.TokenCount,
		Angle:              ocrResp.Angle,
		ScaleFactor:        ocrResp.ScaleFactor,
		BoundingBoxWidth:   ocrResp.BoundingBoxWidth,
		BoundingBoxHeight:  ocrResp.BoundingBoxHeight,
	}

	// Build bounding box records only for text documents
	if ocrResp.IsTextDocument && len(ocrResp.Boxes) > 0 {
		for _, box := range ocrResp.Boxes {
			result.Boxes = append(result.Boxes, domain.OcrBoundingBox{
				X:          box.X,
				Y:          box.Y,
				Width:      box.Width,
				Height:     box.Height,
				Word:       box.Word,
				Confidence: box.Confidence,
			})
		}
	}

	return result
}

// detectContentType determines the MIME content type from file extension
func (om *OcrManager) detectContentType(path string) string {
	if len(path) > 4 {
		ext := path[len(path)-4:]
		if ext == ".png" {
			return "image/png"
		}
	}
	return "image/jpeg"
}
