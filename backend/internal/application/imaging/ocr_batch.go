package imaging

import (
	"log"

	"image-toolkit/internal/domain"

	"gorm.io/gorm"
)

const batchSize = 20

// ClassificationBatch holds pending classifications and their bounding boxes for batch saving
type ClassificationBatch struct {
	Classifications []domain.OcrClassification
	BoxesByImage    map[uint][]domain.OcrBoundingBox
	db              *gorm.DB
}

// NewClassificationBatch creates a new empty batch
func NewClassificationBatch(db *gorm.DB) *ClassificationBatch {
	return &ClassificationBatch{
		BoxesByImage: make(map[uint][]domain.OcrBoundingBox),
		db:           db,
	}
}

// Add adds a classification result to the batch
func (b *ClassificationBatch) Add(result OcrResult) {
	if result.Classification == nil {
		return
	}

	b.Classifications = append(b.Classifications, *result.Classification)
	if len(result.Boxes) > 0 {
		b.BoxesByImage[result.Classification.ImageFileID] = result.Boxes
	}
}

// IsFull returns true if the batch has reached the size limit
func (b *ClassificationBatch) IsFull() bool {
	return len(b.Classifications) >= batchSize
}

// Save persists the batch to the database and resets it
func (b *ClassificationBatch) Save() {
	if len(b.Classifications) == 0 {
		return
	}

	log.Printf("[OCR] Saving batch of %d classifications", len(b.Classifications))

	for i := range b.Classifications {
		classification := &b.Classifications[i]
		if err := b.db.Where("image_file_id = ?", classification.ImageFileID).
			Assign(classification).
			FirstOrCreate(classification).Error; err != nil {
			log.Printf("OCR: failed to save classification for image %d: %v", classification.ImageFileID, err)
			continue
		}

		// Save bounding boxes only for text document classifications
		if classification.IsTextDocument {
			if boxes, ok := b.BoxesByImage[classification.ImageFileID]; ok {
				b.saveBoundingBoxes(classification.ID, boxes)
				delete(b.BoxesByImage, classification.ImageFileID)
			}
		}
	}

	// Reset batch
	b.Classifications = b.Classifications[:0]
}

// saveBoundingBoxes deletes old boxes and inserts new ones for a classification
func (b *ClassificationBatch) saveBoundingBoxes(classificationID uint, boxes []domain.OcrBoundingBox) {
	// Delete old bounding boxes for this classification
	b.db.Where("classification_id = ?", classificationID).Delete(&domain.OcrBoundingBox{})

	for j := range boxes {
		boxes[j].ClassificationID = classificationID
		if err := b.db.Create(&boxes[j]).Error; err != nil {
			log.Printf("OCR: failed to save bounding box for classification %d: %v", classificationID, err)
		}
	}
}
