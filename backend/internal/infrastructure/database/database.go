package database

import (
	"fmt"
	"strconv"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDatabase initializes the database connection and runs migrations
func InitDatabase(cfg *config.AppConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Before AutoMigrate: add alias column as nullable and populate existing data,
	// so GORM doesn't fail trying to add NOT NULL column to a non-empty table.
	db.Exec("ALTER TABLE llm_providers ADD COLUMN IF NOT EXISTS alias VARCHAR(255) DEFAULT ''")

	// Auto-generate aliases for existing records that don't have one
	var unnamedProviders []domain.LlmProvider
	db.Where("alias = '' OR alias IS NULL").Find(&unnamedProviders)
	typeCounts := make(map[string]int)
	for _, p := range unnamedProviders {
		typeCounts[p.Name]++
		alias := p.Name + "_" + strconv.Itoa(typeCounts[p.Name])
		db.Model(&domain.LlmProvider{}).Where("id = ?", p.ID).Update("alias", alias)
		// Update ActiveProvider in LlmSettings if it references old name format
		db.Model(&domain.LlmSettings{}).Where("active_provider = ?", p.Name).Update("active_provider", alias)
	}

	// Now run AutoMigrate — alias column already exists so GORM won't try to add it
	if err := db.AutoMigrate(
		&domain.ImageFile{},
		&domain.GalleryFolder{},
		&domain.AppSettings{},
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
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Drop legacy enabled column from llm_providers (LLM is always active now)
	db.Exec("ALTER TABLE llm_providers DROP COLUMN IF EXISTS enabled")

	// Drop old unique index on name column — Name is now a type, not a unique identifier
	db.Exec("DROP INDEX IF EXISTS idx_llm_providers_name")

	// Set NOT NULL constraint and unique index on alias column after data migration
	db.Exec("ALTER TABLE llm_providers ALTER COLUMN alias SET NOT NULL")
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
