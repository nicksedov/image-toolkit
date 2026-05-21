package helpers

import (
	"image-toolkit/internal/domain"

	"gorm.io/gorm"
)

// SettingsLoader handles loading singleton settings from the database.
type SettingsLoader struct {
	db *gorm.DB
}

// NewSettingsLoader creates a new SettingsLoader.
func NewSettingsLoader(db *gorm.DB) *SettingsLoader {
	return &SettingsLoader{db: db}
}

// AppSettings loads the application settings, returning zero-value defaults if not found.
func (sl *SettingsLoader) AppSettings() domain.AppSettings {
	var settings domain.AppSettings
	if result := sl.db.First(&settings, 1); result.Error != nil {
		return domain.AppSettings{
			ID:                    1,
			OcrConcurrentRequests: 4,
			DailySyncEnabled:      true,
			DailySyncHour:         3,
			DailySyncMinute:       30,
		}
	}
	return settings
}

// AppSettingsIfExists loads application settings, returning false if not found.
func (sl *SettingsLoader) AppSettingsIfExists() (domain.AppSettings, bool) {
	var settings domain.AppSettings
	result := sl.db.First(&settings, 1)
	return settings, result.Error == nil
}

// LlmSettings loads LLM settings, returning zero-value defaults if not found.
func (sl *SettingsLoader) LlmSettings() domain.LlmSettings {
	var settings domain.LlmSettings
	if err := sl.db.First(&settings).Error; err != nil {
		return domain.LlmSettings{
			ActiveProvider: "ollama",
		}
	}
	return settings
}

// LlmSettingsIfExists loads LLM settings, returning false if not found.
func (sl *SettingsLoader) LlmSettingsIfExists() (domain.LlmSettings, bool) {
	var settings domain.LlmSettings
	err := sl.db.First(&settings).Error
	return settings, err == nil
}

// LlmProvider loads settings for a specific provider by name.
func (sl *SettingsLoader) LlmProvider(name string) (domain.LlmProvider, bool) {
	var provider domain.LlmProvider
	err := sl.db.Where("name = ?", name).First(&provider).Error
	return provider, err == nil
}

// AllLlmProviders loads all LLM providers ordered by name.
func (sl *SettingsLoader) AllLlmProviders() []domain.LlmProvider {
	var providers []domain.LlmProvider
	sl.db.Order("name").Find(&providers)
	return providers
}
