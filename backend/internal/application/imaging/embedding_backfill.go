package imaging

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// EmbeddingBackfillProgress holds the current backfill progress.
type EmbeddingBackfillProgress struct {
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Remaining int    `json:"remaining"`
	LastError string `json:"lastError"`
}

// EmbeddingBackfillStatus holds the current status of the embedding backfill.
type EmbeddingBackfillStatus struct {
	Running   bool                       `json:"running"`
	Progress  EmbeddingBackfillProgress  `json:"progress"`
}

// EmbeddingBackfillManager generates embeddings for images that have tags but no embeddings.
type EmbeddingBackfillManager struct {
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	db       *gorm.DB
	progress EmbeddingBackfillProgress
}

// NewEmbeddingBackfillManager creates a new embedding backfill manager.
func NewEmbeddingBackfillManager(db *gorm.DB) *EmbeddingBackfillManager {
	return &EmbeddingBackfillManager{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start begins the embedding backfill process in a goroutine.
func (m *EmbeddingBackfillManager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("embedding backfill already running")
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.progress = EmbeddingBackfillProgress{}
	m.mu.Unlock()

	go m.run()
	return nil
}

// Stop stops the embedding backfill process.
func (m *EmbeddingBackfillManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()
}

// IsRunning returns whether the backfill is currently running.
func (m *EmbeddingBackfillManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// GetStatus returns the current backfill status.
func (m *EmbeddingBackfillManager) GetStatus() EmbeddingBackfillStatus {
	m.mu.Lock()
	running := m.running
	progress := m.progress
	m.mu.Unlock()
	return EmbeddingBackfillStatus{
		Running:  running,
		Progress: progress,
	}
}

// run is the main backfill loop.
func (m *EmbeddingBackfillManager) run() {
	log.Println("Embedding backfill: started")
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		log.Println("Embedding backfill: finished")
	}()

	// Create embedding client
	embeddingClient, modelName, err := resolveEmbeddingClient(m.db)
	if err != nil {
		m.setError(fmt.Sprintf("Failed to create embedding client: %v", err))
		return
	}
	// Count images that need embeddings (have tags but no embedding, or model changed)
	var total int64
	m.db.Table("image_files").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
		Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
		Where("tag_embeddings.id IS NULL OR tag_embeddings.model_name != ?", modelName).
		Distinct("image_files.id").
		Count(&total)

	if total == 0 {
		log.Println("Embedding backfill: all images already have embeddings")
		return
	}

	m.mu.Lock()
	m.progress.Total = int(total)
	m.progress.Remaining = int(total)
	m.mu.Unlock()

	// Process in batches of 100
	const batchSize = 100
	cursor := uint(0)

	for {
		// Check stop signal
		select {
		case <-m.stopCh:
			log.Println("Embedding backfill: stopped by user")
			return
		default:
		}

		// Find next batch of images that need embeddings
		type imageWithTags struct {
			ImageFileID uint
			Tags        string
		}

		var results []imageWithTags
		err := m.db.Table("image_files").
			Select("image_files.id as image_file_id").
			Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
			Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
			Where("(tag_embeddings.id IS NULL OR tag_embeddings.model_name != ?) AND image_files.id > ?", modelName, cursor).
			Group("image_files.id").
			Order("image_files.id ASC").
			Limit(batchSize).
			Find(&results).Error

		if err != nil || len(results) == 0 {
			log.Printf("Embedding backfill: no more images to process (cursor=%d)", cursor)
			break
		}

		// Load tags for each image and build text strings
		imageIDs := make([]uint, len(results))
		for i, r := range results {
			imageIDs[i] = r.ImageFileID
		}

		tagTexts := make([]string, len(imageIDs))
		for i, imgID := range imageIDs {
			var tags []domain.ImageTag
			m.db.Where("image_file_id = ?", imgID).Find(&tags)
			tagStrs := make([]string, len(tags))
			for j, t := range tags {
				tagStrs[j] = strings.ToLower(t.Tag)
			}
			sort.Strings(tagStrs)
			tagTexts[i] = strings.Join(tagStrs, ", ")
		}

		// Call embedding API for the batch
		embeddings, err := embeddingClient.Embed(tagTexts)
		if err != nil {
			m.setError(fmt.Sprintf("Embedding API failed: %v", err))
			log.Printf("Embedding backfill: embedding API error: %v", err)
			// Update cursor to skip this batch and continue
			cursor = imageIDs[len(imageIDs)-1]
			continue
		}

		// Upsert embeddings for each image
		for i, imgID := range imageIDs {
			if i >= len(embeddings) {
				break
			}
			vecStr := llm.Float32SliceToPgVector(embeddings[i])
			tagCount := strings.Count(tagTexts[i], ",") + 1
			if tagTexts[i] == "" {
				tagCount = 0
			}

			// Upsert: delete existing, then create
			m.db.Where("image_file_id = ?", imgID).Delete(&domain.TagEmbedding{})
			embedding := domain.TagEmbedding{
				ImageFileID: imgID,
				Embedding:   vecStr,
				TagCount:    tagCount,
				ModelName:   modelName,
			}
			if err := m.db.Create(&embedding).Error; err != nil {
				log.Printf("Embedding backfill: failed to save embedding for image %d: %v", imgID, err)
			}
		}

		// Update cursor and progress
		cursor = imageIDs[len(imageIDs)-1]
		m.mu.Lock()
		m.progress.Processed += len(imageIDs)
		m.progress.Remaining = m.progress.Total - m.progress.Processed
		m.mu.Unlock()

		log.Printf("Embedding backfill: processed %d/%d images", m.progress.Processed, m.progress.Total)
	}
}

// resolveEmbeddingClient loads LLM settings and creates an EmbeddingClient.
// Shared by the backfill manager and the real-time embedding hook.
func resolveEmbeddingClient(db *gorm.DB) (llm.EmbeddingClient, string, error) {
	var settings domain.LlmSettings
	if err := db.First(&settings).Error; err != nil {
		return nil, "", fmt.Errorf("LLM settings not found")
	}

	providerAlias := settings.EmbeddingProviderAlias
	if providerAlias == "" {
		providerAlias = settings.ActiveProvider
	}

	var provider domain.LlmProvider
	if err := db.Where("alias = ?", providerAlias).First(&provider).Error; err != nil {
		return nil, "", fmt.Errorf("embedding provider '%s' not found", providerAlias)
	}

	modelName := settings.EmbeddingModel
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	client, err := llm.NewEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create embedding client: %w", err)
	}
	return client, modelName, nil
}

// setError updates the last error in progress.
func (m *EmbeddingBackfillManager) setError(msg string) {
	m.mu.Lock()
	m.progress.LastError = msg
	m.mu.Unlock()
}

// GenerateAndSaveEmbedding generates and saves an embedding for a single image's tags.
// Called from the real-time hook after tags are saved.
func GenerateAndSaveEmbedding(db *gorm.DB, imageFileID uint, tags []string) {
	if len(tags) == 0 {
		return
	}

	embeddingClient, modelName, err := resolveEmbeddingClient(db)
	if err != nil {
		log.Printf("Embedding hook: %v", err)
		return
	}

	// Build sorted, lowercased tag string
	tagStrs := make([]string, len(tags))
	for i, t := range tags {
		tagStrs[i] = strings.ToLower(t)
	}
	sort.Strings(tagStrs)
	text := strings.Join(tagStrs, ", ")

	embeddings, err := embeddingClient.Embed([]string{text})
	if err != nil {
		log.Printf("Embedding hook: embed API failed for image %d: %v", imageFileID, err)
		return
	}

	if len(embeddings) == 0 {
		return
	}

	vecStr := llm.Float32SliceToPgVector(embeddings[0])

	// Upsert
	db.Where("image_file_id = ?", imageFileID).Delete(&domain.TagEmbedding{})
	embedding := domain.TagEmbedding{
		ImageFileID: imageFileID,
		Embedding:   vecStr,
		TagCount:    len(tags),
		ModelName:   modelName,
	}
	if err := db.Create(&embedding).Error; err != nil {
		log.Printf("Embedding hook: failed to save embedding for image %d: %v", imageFileID, err)
	}
}
