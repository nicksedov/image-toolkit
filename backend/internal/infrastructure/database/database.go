package database

import (
	"fmt"

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
		&domain.LlmSettings{},
		&domain.OcrLlmRecognition{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Seed default settings row if not exists
	var count int64
	db.Model(&domain.AppSettings{}).Count(&count)
	if count == 0 {
		db.Create(&domain.AppSettings{ID: 1, Theme: "light-purple", Language: "en"})
	}

	// Seed default LLM settings row if not exists
	var llmCount int64
	db.Model(&domain.LlmSettings{}).Count(&llmCount)
	if llmCount == 0 {
		db.Create(&domain.LlmSettings{
			ID:       1,
			Provider: "ollama",
			ApiUrl:   "http://localhost:11434",
			Model:    "minicpm-v",
			Enabled:  false,
		})
	}

	return db, nil
}
