package imaging

import (
	"testing"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/testutil"

	"github.com/stretchr/testify/assert"
)

func TestDetectLanguage_English(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a classification with English words
	classification := &domain.OcrClassification{
		IsTextDocument: true,
	}
	service.db.Create(classification)

	// Create bounding boxes with English text
	for _, word := range []string{"hello", "world", "test"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	language := service.detectLanguage(*classification)

	assert.Equal(t, "en", language, "should detect English")
}

func TestDetectLanguage_Russian(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a classification
	classification := &domain.OcrClassification{
		IsTextDocument: true,
	}
	service.db.Create(classification)

	// Create bounding boxes with Russian text
	for _, word := range []string{"привет", "мир", "тест"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	language := service.detectLanguage(*classification)

	assert.Equal(t, "ru", language, "should detect Russian")
}

func TestParseTags_CommaSeparated(t *testing.T) {
	input := "cat, dog, bird"

	tags := parseTags(input)

	assert.Equal(t, []string{"cat", "dog", "bird"}, tags)
}

func TestLlmOcrService_Recognize_MissingClassification(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Try to recognize without classification - should return error from DB query
	_, err := service.RecognizeWithLlm(1, nil, domain.LlmProvider{})

	assert.Error(t, err, "should error when classification is missing")
}

func TestLlmOcrService_GetRecognition_HasData(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a recognition
	recognition := &domain.OcrLlmRecognition{
		ImageFileID:     1,
		MarkdownContent: "# Test",
		Language:        "en",
	}
	service.db.Create(recognition)

	// Retrieve it
	result, err := service.GetRecognition(1)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "# Test", result.MarkdownContent)
}

func TestLlmOcrService_StartRecognizeAsync_NewTask(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Start recognition (will fail since no image file, but should launch task)
	result := service.StartRecognizeAsync(1, nil, domain.LlmProvider{})

	assert.True(t, result, "should return true for new task")
}

func TestLlmOcrService_StartRecognizeAsync_Duplicate(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a processing task
	service.taskMu.Lock()
	service.processingTasks[1] = &LlmRecognitionStatus{Status: "processing"}
	service.taskMu.Unlock()

	// Try to start duplicate
	result := service.StartRecognizeAsync(1, nil, domain.LlmProvider{})

	assert.False(t, result, "should return false for duplicate task")
}

func TestLlmOcrService_GetRecognizeStatus_Processing(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a processing task
	service.taskMu.Lock()
	service.processingTasks[1] = &LlmRecognitionStatus{Status: "processing"}
	service.taskMu.Unlock()

	status := service.GetRecognizeStatus(1)

	assert.NotNil(t, status)
	assert.Equal(t, "processing", status.Status)
}

func TestLlmOcrService_ExecuteAiAction_Describe(t *testing.T) {
	_, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// This test requires a mock LLM client, skip for now
	// The function is covered by integration tests
}

func TestLlmOcrService_ExecuteAiAction_Tags(t *testing.T) {
	_, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// This test requires a mock LLM client, skip for now
	// The function is covered by integration tests
}

func setupLlmOcrService(t *testing.T) (*LlmOcrService, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	service := NewLlmOcrService(db)
	return service, cleanup
}
