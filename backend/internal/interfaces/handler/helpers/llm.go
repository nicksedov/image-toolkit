package helpers

import (
	"net/http"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"
	"image-toolkit/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LLMFactory creates LLM clients from database settings.
type LLMFactory struct {
	db                 *gorm.DB
	maxImageMegapixels float64
}

// NewLLMFactory creates a new LLMFactory.
func NewLLMFactory(db *gorm.DB, maxImageMegapixels float64) *LLMFactory {
	return &LLMFactory{db: db, maxImageMegapixels: maxImageMegapixels}
}

// CreateClient creates an LLM client from the current database settings.
// Returns (client, settings, success). If success is false, an error response has been written.
func (f *LLMFactory) CreateClient(c *gin.Context) (llm.Client, domain.LlmSettings, bool) {
	var settings domain.LlmSettings
	if err := f.db.First(&settings).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return nil, domain.LlmSettings{}, false
	}

	if !settings.Enabled {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgLlmOcrNotEnabled))
		return nil, domain.LlmSettings{}, false
	}

	client, err := llm.NewClient(settings.Provider, settings.ApiUrl, settings.ApiKey, settings.Model, f.maxImageMegapixels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return nil, domain.LlmSettings{}, false
	}

	return client, settings, true
}

// GetEnabledSettings returns LLM settings only if enabled, otherwise writes error and returns false.
func (f *LLMFactory) GetEnabledSettings(c *gin.Context) (domain.LlmSettings, bool) {
	var settings domain.LlmSettings
	if err := f.db.First(&settings).Error; err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return domain.LlmSettings{}, false
	}

	if !settings.Enabled {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgLlmOcrNotEnabled))
		return domain.LlmSettings{}, false
	}

	return settings, true
}
