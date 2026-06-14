package database

import (
	"fmt"
	"log"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDatabase initializes the database connection and runs migrations.
func InitDatabase(cfg *config.AppConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// PrepareStmt avoids the simple protocol path in the PostgreSQL migrator
		// (GetRows), which triggers a pgx sanitizer bug with QueryExecModeSimpleProtocol.
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Enable pgvector extension BEFORE AutoMigrate needs the vector type
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return nil, fmt.Errorf("failed to enable pgvector extension: %w", err)
	}

	// Run AutoMigrate
	if err := db.AutoMigrate(
		&domain.ImageFile{},
		&domain.GalleryFolder{},
		&domain.AppSettings{},
		&domain.GeolocationCache{},
		&domain.ImageMetadata{},
		&domain.User{},
		&domain.UserSettings{},
		&domain.Session{},
		&domain.AuditLog{},
		&domain.OcrClassification{},
		&domain.OcrBoundingBox{},
		&domain.LlmProvider{},
		&domain.LlmSettings{},
		&domain.OcrLlmRecognition{},
		&domain.ImageTag{},
		&domain.LlmProviderModelCache{},
		&domain.Conversation{},
		&domain.ConversationMessage{},
		&domain.TagEmbedding{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// One-time migration: move legacy embedding data to per-model child table before dropping columns.
	// Detects if the old 'embedding' column still exists on tag_embeddings and migrates data.
	var hasLegacyEmbeddingCol bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_name = 'tag_embeddings' AND column_name = 'embedding'
	)`).Scan(&hasLegacyEmbeddingCol)

	if hasLegacyEmbeddingCol {
		log.Println("Migration: detected legacy embedding column on tag_embeddings, migrating data...")

		// Determine the model name from existing data
		var existingModel string
		db.Raw("SELECT model_name FROM tag_embeddings WHERE model_name IS NOT NULL AND model_name != '' LIMIT 1").Scan(&existingModel)
		if existingModel == "" {
			existingModel = "qwen3-embedding:4b"
		}

		// Detect actual dimension from the existing column
		var dim int
		db.Raw(`SELECT COALESCE(
			(SELECT atttypmod FROM pg_attribute
			 WHERE attrelid = 'tag_embeddings'::regclass AND attname = 'embedding'), 0)
		`).Scan(&dim)
		if dim == 0 {
			dim = 1024
		}

		// Create the child table and migrate data
		if err := EnsureEmbeddingTable(db, existingModel, dim); err != nil {
			log.Printf("Migration: failed to ensure child table: %v", err)
		} else {
			tableName := domain.EmbeddingTableName(existingModel)
			if err := db.Exec(fmt.Sprintf(`
				INSERT INTO %s (tag_embeddings_id, dimensity, embedding)
				SELECT id, %d, embedding FROM tag_embeddings WHERE embedding IS NOT NULL
				ON CONFLICT DO NOTHING
			`, tableName, dim)).Error; err != nil {
				log.Printf("Migration: failed to copy embedding data to %s: %v", tableName, err)
			} else {
				log.Printf("Migration: copied embedding data to %s", tableName)
			}
		}

		// Drop legacy columns
		if err := db.Exec("ALTER TABLE tag_embeddings DROP COLUMN IF EXISTS embedding").Error; err != nil {
			log.Printf("Migration: failed to drop legacy embedding column: %v", err)
		}
		if err := db.Exec("ALTER TABLE tag_embeddings DROP COLUMN IF EXISTS model_name").Error; err != nil {
			log.Printf("Migration: failed to drop legacy model_name column: %v", err)
		}
		log.Println("Migration: dropped legacy embedding and model_name columns from tag_embeddings")
	}

	// Drop the legacy HNSW index on tag_embeddings if it still exists (no longer needed on parent table)
	db.Exec("DROP INDEX IF EXISTS idx_tag_embeddings_vector")

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_providers_alias ON llm_providers (alias)")

	// Create composite index for calendar pagination: covers ORDER BY date_taken, image_file_id
	db.Exec("CREATE INDEX IF NOT EXISTS idx_image_metadata_date_taken_file_id ON image_metadata (date_taken, image_file_id)")

	// Seed default settings row if not exists
	var count int64
	db.Model(&domain.AppSettings{}).Count(&count)
	if count == 0 {
		db.Create(&domain.AppSettings{ID: 1})
	}

	// Seed default LLM settings row if not exists
	var llmCount int64
	db.Model(&domain.LlmSettings{}).Count(&llmCount)
	if llmCount == 0 {
		db.Create(&domain.LlmSettings{
			ID:             1,
			ActiveProvider: "ollama_1",
		})
	}

	// Seed default LLM providers if not exist
	var providerCount int64
	db.Model(&domain.LlmProvider{}).Count(&providerCount)
	if providerCount == 0 {
		db.Create([]domain.LlmProvider{
			{Name: "ollama", Alias: "ollama_1", ApiUrl: "http://localhost:11434", Model: "minicpm-v"},
			{Name: "ollama_cloud", Alias: "ollama_cloud_1", ApiUrl: "https://ollama.com", Model: "minicpm-v"},
			{Name: "openai", Alias: "openai_1", ApiUrl: "https://api.openai.com", Model: "gpt-4-vision-preview"},
		})
	}

	return db, nil
}
