package imaging

import (
	"testing"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/testutil"
	"image-toolkit/internal/testutil/fixtures"
	"image-toolkit/internal/testutil/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLlmOcrService_Recognize_Success(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Create OCR classification with bounding boxes (for language detection)
	classification := &domain.OcrClassification{
		ImageFileID:    imgFile.ID,
		IsTextDocument: true,
	}
	service.db.Create(classification)

	// Add bounding boxes with English text
	for _, word := range []string{"hello", "world"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	// Mock LLM to return markdown
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("# Document Title\n\nContent goes here."),
	}
	provider := domain.LlmProvider{Name: "ollama", Model: "minicpm-v"}

	result, err := service.RecognizeWithLlm(imgFile.ID, mockClient, provider)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.MarkdownContent, "# Document Title")
	assert.Equal(t, "en", result.Language)
	assert.Equal(t, "ollama", result.Provider)
	assert.Equal(t, "minicpm-v", result.Model)

	// Verify recognition saved to DB
	saved, err := service.GetRecognition(imgFile.ID)
	require.NoError(t, err)
	assert.NotNil(t, saved)
	assert.True(t, saved.Success)
}

func TestLlmOcrService_Recognize_LLMError(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Create OCR classification with bounding boxes
	classification := &domain.OcrClassification{
		ImageFileID:    imgFile.ID,
		IsTextDocument: true,
	}
	service.db.Create(classification)

	for _, word := range []string{"hello", "world"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	// Mock LLM to return error
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.LlmErrorResponse(assert.AnError),
	}
	provider := domain.LlmProvider{Name: "ollama", Model: "minicpm-v"}

	_, err := service.RecognizeWithLlm(imgFile.ID, mockClient, provider)

	require.Error(t, err)

	// Verify failed recognition was saved to DB
	saved, dbErr := service.GetRecognition(imgFile.ID)
	require.NoError(t, dbErr)
	assert.NotNil(t, saved)
	assert.False(t, saved.Success)
	assert.NotEmpty(t, saved.Error)
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
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("A beautiful landscape image with mountains and a lake."),
	}
	provider := domain.LlmProvider{Name: "ollama", Model: "minicpm-v"}

	result, err := service.ExecuteAiAction(imgFile.ID, "describe", "", "en", mockClient, provider)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Result, "landscape")
	assert.Equal(t, "ollama", result.Provider)
}

func TestLlmOcrService_ExecuteAiAction_Tags(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("cat, dog, bird"),
	}
	provider := domain.LlmProvider{Name: "ollama", Model: "minicpm-v"}

	result, err := service.ExecuteAiAction(imgFile.ID, "tags", "", "en", mockClient, provider)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, []string{"cat", "dog", "bird"}, result.Tags)
}

func TestLlmOcrService_StoreCachedTagsResult(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	tags := []string{"cat", "dog", "bird"}
	service.StoreCachedTagsResult("task-1", tags)

	status := service.GetAiActionStatus("task-1")
	require.NotNil(t, status)
	assert.Equal(t, "completed", status.Status)
	assert.Equal(t, "tags", status.Action)
	require.NotNil(t, status.Result)
	assert.Equal(t, tags, status.Result.Tags)

	// Status should be cleaned up after first read
	assert.Nil(t, service.GetAiActionStatus("task-1"))
}

func TestLlmOcrService_SaveImageTags(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	imgFile := testutil.SeedImageFileNoT(service.db, "/tmp/test.jpg", "hash123", 1000)

	// Save initial tags
	err := service.SaveImageTags(imgFile.ID, []string{"cat", "dog"})
	require.NoError(t, err)

	var tags []domain.ImageTag
	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Len(t, tags, 2)

	// Save new tags - should replace old ones
	err = service.SaveImageTags(imgFile.ID, []string{"bird", "fish", "snake"})
	require.NoError(t, err)

	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Len(t, tags, 3)
	tagValues := make([]string, len(tags))
	for i, tag := range tags {
		tagValues[i] = tag.Tag
	}
	assert.Equal(t, []string{"bird", "fish", "snake"}, tagValues)
}

func TestLlmOcrService_SaveImageTags_EmptyTags(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	imgFile := testutil.SeedImageFileNoT(service.db, "/tmp/test.jpg", "hash123", 1000)

	// Save initial tags
	err := service.SaveImageTags(imgFile.ID, []string{"cat", "dog"})
	require.NoError(t, err)

	// Save empty tags - should delete old ones and not insert new
	err = service.SaveImageTags(imgFile.ID, []string{})
	require.NoError(t, err)

	var tags []domain.ImageTag
	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Empty(t, tags)
}

func TestLlmOcrService_StartAiActionAsync_SavesTagsToDB(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("cat, dog, bird"),
	}
	provider := domain.LlmProvider{Name: "ollama", Model: "minicpm-v"}

	service.StartAiActionAsync("task-1", imgFile.ID, "tags", "", "en", mockClient, provider)

	// Wait for async completion
	require.Eventually(t, func() bool {
		s := service.GetAiActionStatus("task-1")
		return s != nil && s.Status != "processing"
	}, 5*time.Second, 50*time.Millisecond)

	// Verify tags were saved to DB
	var tags []domain.ImageTag
	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Len(t, tags, 3)
}

func setupLlmOcrService(t *testing.T) (*LlmOcrService, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	service := NewLlmOcrService(db)
	return service, cleanup
}
