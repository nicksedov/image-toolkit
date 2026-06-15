package database

import (
	"fmt"
	"regexp"

	"image-toolkit/internal/domain"

	"gorm.io/gorm"
)

// validTableName matches a valid PostgreSQL table name for embedding child tables.
var validTableName = regexp.MustCompile(`^tag_embeddings_[a-zA-Z0-9_]+$`)

// EnsureEmbeddingTable creates the per-model child table and indexes if they don't exist.
// The child table stores vector embeddings for a specific model and references tag_embeddings.
// Uses pgvector's halfvec type (fp16) which supports HNSW indexing up to 4000 dimensions.
func EnsureEmbeddingTable(db *gorm.DB, modelName string, dimension int) error {
	tableName := domain.EmbeddingTableName(modelName)

	// Validate the generated table name to prevent SQL injection
	if !validTableName.MatchString(tableName) {
		return fmt.Errorf("invalid embedding table name: %s (from model: %s)", tableName, modelName)
	}

	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			tag_embeddings_id BIGINT NOT NULL REFERENCES tag_embeddings(id) ON DELETE CASCADE,
			dimensity INT NOT NULL DEFAULT %d,
			embedding halfvec(%d) NOT NULL
		)`, tableName, dimension, dimension)

	if err := db.Exec(createSQL).Error; err != nil {
		return fmt.Errorf("failed to create embedding table %s: %w", tableName, err)
	}

	// Create HNSW index for fast vector similarity search (halfvec_cosine_ops for fp16)
	indexSQL := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING hnsw (embedding halfvec_cosine_ops)",
		tableName, tableName)
	if err := db.Exec(indexSQL).Error; err != nil {
		return fmt.Errorf("failed to create HNSW index on %s: %w", tableName, err)
	}

	// Unique constraint: one embedding per image per model
	constraintSQL := fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_tag_embeddings_id ON %s (tag_embeddings_id)",
		tableName, tableName)
	if err := db.Exec(constraintSQL).Error; err != nil {
		return fmt.Errorf("failed to create unique index on %s: %w", tableName, err)
	}

	return nil
}

// EmbeddingTableExists checks if a per-model embedding child table exists in the database.
func EmbeddingTableExists(db *gorm.DB, modelName string) bool {
	tableName := domain.EmbeddingTableName(modelName)
	var exists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)", tableName).Scan(&exists)
	return exists
}
