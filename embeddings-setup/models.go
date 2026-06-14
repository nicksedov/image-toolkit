package main

import "time"

// ImageTag represents a single AI-generated tag for an image.
// Source: backend/internal/domain/media.go — ImageTag struct.
type ImageTag struct {
	ID          uint   `gorm:"primaryKey"`
	ImageFileID uint   `gorm:"index;not null"`
	Tag         string `gorm:"not null"`
}

// TagEmbedding stores vector embeddings for semantic tag search (pgvector).
// One embedding per image, generated from concatenated AI tags.
// Source: backend/internal/domain/media.go — TagEmbedding struct.
type TagEmbedding struct {
	ID          uint      `gorm:"primaryKey"`
	ImageFileID uint      `gorm:"uniqueIndex;not null"`
	Embedding   string    `gorm:"type:vector(1024);not null"`
	TagCount    int       `gorm:"not null"`
	ModelName   string    `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// LlmSettings holds LLM provider and embedding configuration.
// Source: backend/internal/domain/media.go — LlmSettings struct.
type LlmSettings struct {
	ID                     uint      `gorm:"primaryKey"`
	ActiveProvider         string    `gorm:"default:ollama_1;not null"`
	EmbeddingProviderAlias string    `gorm:"default:''"`
	EmbeddingModel         string    `gorm:"default:'qwen3-embedding:4b'"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// LlmProvider represents a configured LLM provider instance.
// Source: backend/internal/domain/media.go — LlmProvider struct.
type LlmProvider struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"index;not null"`
	Alias     string    `gorm:"not null"`
	ApiUrl    string    `gorm:"not null"`
	ApiKey    string    `gorm:"default:''"`
	Model     string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
