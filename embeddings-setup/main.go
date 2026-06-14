// embeddings-setup is a standalone CLI utility that populates the
// tag_embeddings table from existing image_tags records.
//
// Usage:
//
//	go run . --dry-run          # count images needing embeddings
//	go run .                  # process all with default batch size (100)
//	go run . --batch-size 50  # smaller batches if API timeouts occur
package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

func main() {
	batchSize := flag.Int("batch-size", 100, "Number of images per embedding API call")
	dryRun := flag.Bool("dry-run", false, "Count images needing embeddings without processing")
	flag.Parse()

	// Load backend/.env and connect to PostgreSQL
	loadEnv()
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Println("Connected to database")

	// Resolve embedding provider and model from llm_settings + llm_providers
	embeddingClient, modelName, err := resolveEmbeddingClient(db)
	if err != nil {
		log.Fatalf("Failed to create embedding client: %v", err)
	}
	log.Printf("Embedding provider ready — model: %s", modelName)

	// Count total images that need embeddings (have tags but no embedding, or model changed)
	var total int64
	db.Table("image_files").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
		Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
		Where("tag_embeddings.id IS NULL OR tag_embeddings.model_name != ?", modelName).
		Distinct("image_files.id").
		Count(&total)

	if total == 0 {
		log.Println("All images already have up-to-date embeddings — nothing to do")
		return
	}

	log.Printf("Found %d image(s) needing embeddings", total)

	if *dryRun {
		fmt.Printf("\n--dry-run: %d image(s) would be processed (batch size: %d)\n", total, *batchSize)
		return
	}

	// Cursor-based batch loop
	processed := 0
	cursor := uint(0)
	startTime := time.Now()

	for {
		// Fetch next batch of image_file_ids that need embeddings
		var imageIDs []uint
		err := db.Table("image_files").
			Select("DISTINCT image_files.id").
			Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
			Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
			Where("(tag_embeddings.id IS NULL OR tag_embeddings.model_name != ?) AND image_files.id > ?", modelName, cursor).
			Order("image_files.id ASC").
			Limit(*batchSize).
			Pluck("image_files.id", &imageIDs).Error
		if err != nil {
			log.Fatalf("Failed to fetch image batch: %v", err)
		}
		if len(imageIDs) == 0 {
			break
		}

		// Build tag texts for each image in the batch
		tagTexts := make([]string, len(imageIDs))
		for i, imgID := range imageIDs {
			var tags []ImageTag
			db.Where("image_file_id = ?", imgID).Find(&tags)
			parts := make([]string, len(tags))
			for j, t := range tags {
				parts[j] = strings.ToLower(t.Tag)
			}
			sort.Strings(parts)
			tagTexts[i] = strings.Join(parts, ", ")
		}

		// Call embedding API for the whole batch
		embeddings, err := embeddingClient.Embed(tagTexts)
		if err != nil {
			log.Printf("Embedding API failed for batch (cursor=%d): %v — skipping batch", cursor, err)
			cursor = imageIDs[len(imageIDs)-1]
			continue
		}

		// Upsert embeddings for each image
		for i, imgID := range imageIDs {
			if i >= len(embeddings) {
				break
			}
			vecStr := float32SliceToPgVector(embeddings[i])
			tagCount := 0
			if tagTexts[i] != "" {
				tagCount = strings.Count(tagTexts[i], ",") + 1
			}

			// Upsert: delete existing then insert (matches backend pattern)
			db.Where("image_file_id = ?", imgID).Delete(&TagEmbedding{})
			embedding := TagEmbedding{
				ImageFileID: imgID,
				Embedding:   vecStr,
				TagCount:    tagCount,
				ModelName:   modelName,
			}
			if err := db.Create(&embedding).Error; err != nil {
				log.Printf("Failed to save embedding for image %d: %v", imgID, err)
			}
		}

		processed += len(imageIDs)
		cursor = imageIDs[len(imageIDs)-1]
		elapsed := time.Since(startTime).Round(time.Second)
		pct := float64(processed) / float64(total) * 100
		log.Printf("Progress: %d/%d (%.1f%%) — elapsed %s", processed, total, pct, elapsed)
	}

	log.Printf("Done — %d embedding(s) generated in %s", processed, time.Since(startTime).Round(time.Second))
}

// resolveEmbeddingClient reads llm_settings and llm_providers from the DB
// and creates an EmbeddingClient for the configured provider and model.
func resolveEmbeddingClient(db *gorm.DB) (EmbeddingClient, string, error) {
	var settings LlmSettings
	if err := db.First(&settings).Error; err != nil {
		return nil, "", fmt.Errorf("llm_settings not found: %w", err)
	}

	providerAlias := settings.EmbeddingProviderAlias
	if providerAlias == "" {
		providerAlias = settings.ActiveProvider
	}

	var provider LlmProvider
	if err := db.Where("alias = ?", providerAlias).First(&provider).Error; err != nil {
		return nil, "", fmt.Errorf("embedding provider '%s' not found in llm_providers: %w", providerAlias, err)
	}

	modelName := settings.EmbeddingModel
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	client, err := newEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create embedding client: %w", err)
	}
	return client, modelName, nil
}
