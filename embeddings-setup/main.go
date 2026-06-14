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
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// sanitizeModelName converts an embedding model name to a valid PostgreSQL table name suffix.
var nonAlphanumUnderscore = regexp.MustCompile(`[^a-zA-Z0-9_]`)
var multiUnderscore = regexp.MustCompile(`_+`)

func sanitizeModelName(modelName string) string {
	s := nonAlphanumUnderscore.ReplaceAllString(modelName, "_")
	s = multiUnderscore.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func embeddingTableName(modelName string) string {
	return "tag_embeddings_" + sanitizeModelName(modelName)
}

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
	embeddingClient, modelName, _, err := resolveEmbeddingClient(db)
	if err != nil {
		log.Fatalf("Failed to create embedding client: %v", err)
	}
	log.Printf("Embedding provider ready — model: %s", modelName)

	// Probe the embedding model to detect actual dimension.
	log.Println("Probing embedding model to detect vector dimension...")
	probe, err := embeddingClient.Embed([]string{"dimension probe"})
	if err != nil || len(probe) == 0 {
		log.Fatalf("Failed to probe embedding model: %v", err)
	}
	actualDim := len(probe[0])
	log.Printf("Embedding model produces %d-dimensional vectors", actualDim)

	// Ensure the per-model child table exists with the correct dimension
	childTable := embeddingTableName(modelName)
	log.Printf("Ensuring child table %s exists with vector(%d)...", childTable, actualDim)

	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			tag_embeddings_id BIGINT NOT NULL REFERENCES tag_embeddings(id) ON DELETE CASCADE,
			dimensity INT NOT NULL DEFAULT %d,
			embedding vector(%d) NOT NULL
		)`, childTable, actualDim, actualDim)
	if err := db.Exec(createSQL).Error; err != nil {
		log.Fatalf("Failed to create child table %s: %v", childTable, err)
	}

	// Create HNSW index on the child table
	db.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING hnsw (embedding vector_cosine_ops)", childTable, childTable))
	// Unique constraint on tag_embeddings_id
	db.Exec(fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_tag_embeddings_id ON %s (tag_embeddings_id)", childTable, childTable))
	log.Println("Child table and indexes ensured")

	// Count total images that need embeddings for this model
	var total int64
	db.Table("image_files").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
		Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
		Joins(fmt.Sprintf("LEFT JOIN %s ON %s.tag_embeddings_id = tag_embeddings.id", childTable, childTable)).
		Where(fmt.Sprintf("%s.id IS NULL", childTable)).
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
			Joins(fmt.Sprintf("LEFT JOIN %s ON %s.tag_embeddings_id = tag_embeddings.id", childTable, childTable)).
			Where(fmt.Sprintf("%s.id IS NULL AND image_files.id > ?", childTable), cursor).
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

			// Upsert parent record
			var parent TagEmbedding
			result := db.Where("image_file_id = ?", imgID).First(&parent)
			if result.Error != nil {
				parent = TagEmbedding{ImageFileID: imgID, TagCount: tagCount}
				if err := db.Create(&parent).Error; err != nil {
					log.Printf("Failed to create parent embedding for image %d: %v", imgID, err)
					continue
				}
			} else {
				db.Model(&parent).Update("tag_count", tagCount)
			}

			// Upsert child record (delete + insert)
			db.Exec(fmt.Sprintf("DELETE FROM %s WHERE tag_embeddings_id = ?", childTable), parent.ID)
			if err := db.Exec(fmt.Sprintf(
				"INSERT INTO %s (tag_embeddings_id, dimensity, embedding) VALUES (?, ?, ?::vector)",
				childTable), parent.ID, actualDim, vecStr).Error; err != nil {
				log.Printf("Failed to save child embedding for image %d: %v", imgID, err)
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
// Returns the client, model name, embedding dimension, and any error.
func resolveEmbeddingClient(db *gorm.DB) (EmbeddingClient, string, int, error) {
	var settings LlmSettings
	if err := db.First(&settings).Error; err != nil {
		return nil, "", 0, fmt.Errorf("llm_settings not found: %w", err)
	}

	providerAlias := settings.EmbeddingProviderAlias
	if providerAlias == "" {
		providerAlias = settings.ActiveProvider
	}

	var provider LlmProvider
	if err := db.Where("alias = ?", providerAlias).First(&provider).Error; err != nil {
		return nil, "", 0, fmt.Errorf("embedding provider '%s' not found in llm_providers: %w", providerAlias, err)
	}

	modelName := settings.EmbeddingModel
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	dimension := settings.EmbeddingDimension
	if dimension == 0 {
		dimension = 1024
	}

	client, err := newEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to create embedding client: %w", err)
	}
	return client, modelName, dimension, nil
}
