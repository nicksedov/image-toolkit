package imaging

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// LlmRecognitionStatus represents the status of an async recognition task
type LlmRecognitionStatus struct {
	Status string           // "processing", "completed", "failed"
	Result *RecognizeResult // non-nil when Status == "completed"
	Error  string           // non-empty when Status == "failed"
}

// LlmOcrService handles VL LLM-based OCR recognition
type LlmOcrService struct {
	db              *gorm.DB
	processingTasks map[uint]*LlmRecognitionStatus
	taskMu          sync.Mutex
}

// NewLlmOcrService creates a new LLM OCR service
func NewLlmOcrService(db *gorm.DB) *LlmOcrService {
	return &LlmOcrService{
		db:              db,
		processingTasks: make(map[uint]*LlmRecognitionStatus),
	}
}

// RecognizeResult holds the result of LLM recognition
type RecognizeResult struct {
	Success          bool
	MarkdownContent  string
	Language         string
	Provider         string
	Model            string
	ProcessingTimeMs int
	Error            string
}

// RecognizeWithLlm performs OCR using VL LLM
func (s *LlmOcrService) RecognizeWithLlm(imageFileID uint, client llm.Client, settings domain.LlmSettings) (*RecognizeResult, error) {
	// Step 1: Get OCR classification to detect language
	var classification domain.OcrClassification
	if err := s.db.Where("image_file_id = ?", imageFileID).First(&classification).Error; err != nil {
		return nil, fmt.Errorf("failed to get OCR classification: %w", err)
	}

	// Step 2: Detect language from classification data
	language := s.detectLanguage(classification)

	// Step 3: Build system prompt
	systemPrompt := buildOcrSystemPrompt(language)

	// Step 4: Get image path
	var imageFile domain.ImageFile
	if err := s.db.First(&imageFile, imageFileID).Error; err != nil {
		return nil, fmt.Errorf("failed to get image file: %w", err)
	}

	// Step 5: Call LLM
	startTime := time.Now()
	markdownContent, err := client.Recognize(imageFile.Path, systemPrompt)
	processingTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		// Save failed result using UPSERT
		recognition := domain.OcrLlmRecognition{
			ImageFileID:         imageFileID,
			OcrClassificationID: classification.ID,
			Language:            language,
			MarkdownContent:     "",
			Provider:            settings.Provider,
			Model:               settings.Model,
			ProcessingTimeMs:    processingTime,
			Error:               err.Error(),
			Success:             false,
		}
		s.db.
			Where("image_file_id = ?", imageFileID).
			Assign(recognition).
			FirstOrCreate(&recognition)
		return nil, err
	}

	// Step 6: Save successful result using UPSERT
	recognition := domain.OcrLlmRecognition{
		ImageFileID:         imageFileID,
		OcrClassificationID: classification.ID,
		Language:            language,
		MarkdownContent:     markdownContent,
		Provider:            settings.Provider,
		Model:               settings.Model,
		ProcessingTimeMs:    processingTime,
		Error:               "",
		Success:             true,
	}
	if err := s.db.
		Where("image_file_id = ?", imageFileID).
		Assign(recognition).
		FirstOrCreate(&recognition).Error; err != nil {
		return nil, fmt.Errorf("failed to save recognition result: %w", err)
	}

	return &RecognizeResult{
		Success:          true,
		MarkdownContent:  markdownContent,
		Language:         language,
		Provider:         settings.Provider,
		Model:            settings.Model,
		ProcessingTimeMs: processingTime,
		Error:            "",
	}, nil
}

// StartRecognizeAsync starts LLM recognition in a background goroutine.
// Returns true if a new task was started, false if already processing.
func (s *LlmOcrService) StartRecognizeAsync(imageFileID uint, client llm.Client, settings domain.LlmSettings) bool {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	if task, exists := s.processingTasks[imageFileID]; exists && task.Status == "processing" {
		return false // already processing
	}

	s.processingTasks[imageFileID] = &LlmRecognitionStatus{Status: "processing"}

	go func() {
		result, err := s.RecognizeWithLlm(imageFileID, client, settings)

		s.taskMu.Lock()
		defer s.taskMu.Unlock()

		if err != nil {
			s.processingTasks[imageFileID] = &LlmRecognitionStatus{
				Status: "failed",
				Error:  err.Error(),
			}
		} else {
			s.processingTasks[imageFileID] = &LlmRecognitionStatus{
				Status: "completed",
				Result: result,
			}
		}
	}()

	return true
}

// GetRecognizeStatus returns the current async task status for an image.
// Returns nil if no task exists for this image.
func (s *LlmOcrService) GetRecognizeStatus(imageFileID uint) *LlmRecognitionStatus {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	task, exists := s.processingTasks[imageFileID]
	if !exists {
		return nil
	}

	// Clean up completed/failed tasks after reading
	if task.Status != "processing" {
		delete(s.processingTasks, imageFileID)
	}

	return task
}

// GetRecognition retrieves LLM OCR recognition for an image
func (s *LlmOcrService) GetRecognition(imageFileID uint) (*domain.OcrLlmRecognition, error) {
	var recognition domain.OcrLlmRecognition
	if err := s.db.Where("image_file_id = ?", imageFileID).Order("id DESC").First(&recognition).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &recognition, nil
}

// detectLanguage determines document language from OCR classification data
func (s *LlmOcrService) detectLanguage(classification domain.OcrClassification) string {
	// Get bounding boxes to analyze words for language detection
	var boxes []domain.OcrBoundingBox
	s.db.Where("classification_id = ?", classification.ID).Find(&boxes)

	if len(boxes) == 0 {
		return "en" // Default to English
	}

	// Simple language detection based on character sets
	russianCount := 0
	englishCount := 0

	for _, box := range boxes {
		word := strings.ToLower(box.Word)
		for _, ch := range word {
			// Cyrillic range
			if ch >= 0x0400 && ch <= 0x04FF {
				russianCount++
			}
			// Latin range
			if ch >= 0x0061 && ch <= 0x007A {
				englishCount++
			}
		}
	}

	if russianCount > englishCount {
		return "ru"
	}
	return "en"
}

// buildOcrSystemPrompt creates the system prompt for VL LLM
func buildOcrSystemPrompt(language string) string {
	langName := "English"
	if language == "ru" {
		langName = "Russian"
	}

	return fmt.Sprintf(`You are an OCR model. Convert the document in %s language to markdown format, preserving formatting for headings, paragraphs, tables, and formulas.

Rules:
1. Output ONLY the document content - NO comments, explanations, or meta-text
2. Preserve the original structure: headings, lists, tables, formulas
3. Use proper markdown syntax (# for H1, ## for H2, etc.)
4. For tables, use markdown table format (| column | column |)
5. For formulas, use LaTeX format ($formula$ for inline, $$formula$$ for block)
6. Maintain the original language - do NOT translate
7. If text is unclear, mark it as [unclear]
8. Do NOT add "Here is the document" or "This document contains" - just output the content

Return only the markdown content, nothing else.`, langName)
}

// AiActionResult holds the result of an AI action
type AiActionResult struct {
	Success          bool
	Result           string
	Tags             []string
	Provider         string
	Model            string
	ProcessingTimeMs int
	Error            string
}

// ExecuteAiAction performs an AI action (describe, tags, recognizeText, askQuestion)
func (s *LlmOcrService) ExecuteAiAction(imageFileID uint, action string, question string, client llm.Client, settings domain.LlmSettings) (*AiActionResult, error) {
	// Get image file
	var imageFile domain.ImageFile
	if err := s.db.First(&imageFile, imageFileID).Error; err != nil {
		return nil, fmt.Errorf("failed to get image file: %w", err)
	}

	// Build system prompt based on action
	systemPrompt := buildAiActionPrompt(action, question)

	// Call LLM
	startTime := time.Now()
	response, err := client.Recognize(imageFile.Path, systemPrompt)
	processingTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return nil, fmt.Errorf("failed to execute AI action: %w", err)
	}

	result := &AiActionResult{
		Success:          true,
		Provider:         settings.Provider,
		Model:            settings.Model,
		ProcessingTimeMs: processingTime,
	}

	// Parse response based on action
	switch action {
	case "tags":
		// Split comma-separated or newline-separated tags
		tags := parseTags(response)
		result.Tags = tags
		result.Result = response
	default:
		result.Result = response
	}

	return result, nil
}

// buildAiActionPrompt creates the system prompt for AI actions
func buildAiActionPrompt(action string, question string) string {
	switch action {
	case "describe":
		return `You are an AI assistant that describes images in detail. Describe what you see in the image, including:
- Main objects and subjects
- Background and setting
- Colors and lighting
- Actions or activities
- Mood or atmosphere

Provide a comprehensive description in natural language. Be specific and detailed.`

	case "tags":
		return `You are an AI assistant that generates relevant tags for images. Analyze the image and provide a list of descriptive tags for:
- Objects and items visible
- Actions or activities
- Setting and environment
- Mood and atmosphere
- Colors and styles
- Categories and themes

Return the tags as a comma-separated list. Use single words or short phrases. Do NOT include explanations, just the tags.`

	case "recognizeText":
		return `You are an OCR (Optical Character Recognition) model. Extract and recognize all text from this image.

Rules:
1. Extract ALL visible text in the image
2. Preserve the original language and formatting where possible
3. If text is unclear, mark it as [unclear]
4. Return ONLY the recognized text, no explanations
5. If there is no text in the image, respond with "No text detected."

Recognize and output the text content.`

	case "askQuestion":
		return fmt.Sprintf(`You are an AI assistant that answers questions about images. The user will ask you a question about this image.

Question: %s

Provide a clear and helpful answer based on what you can see in the image. Be specific and accurate. If you cannot determine the answer from the image, say so clearly.`, question)

	default:
		return "Analyze this image and provide your observations."
	}
}

// parseTags parses a comma-separated or newline-separated list of tags
func parseTags(input string) []string {
	// Split by comma first
	parts := strings.Split(input, ",")

	// If only one part, try splitting by newline
	if len(parts) == 1 {
		parts = strings.Split(input, "\n")
	}

	var tags []string
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		// Remove leading/trailing quotes
		tag = strings.Trim(tag, `"'`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}
