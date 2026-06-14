package imaging

import (
	"testing"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/testutil"
	"image-toolkit/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBackgroundSyncManager(t *testing.T) (*BackgroundSyncManager, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	// Create manager without thumbnail service or geocoder (nil)
	manager := NewBackgroundSyncManager(db, nil, nil)
	return manager, cleanup
}

func TestBackgroundSyncManager_Start(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	bsm.Start(true, 14, 0)

	time.Sleep(50 * time.Millisecond)
	assert.True(t, bsm.isRunning(), "sync should be running")

	bsm.Stop()
}

func TestBackgroundSyncManager_Stop(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	bsm.Start(true, 14, 0)
	bsm.Stop()

	time.Sleep(50 * time.Millisecond)
	assert.False(t, bsm.isRunning(), "sync should not be running")
}

func TestBackgroundSyncManager_isRunning(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	assert.False(t, bsm.isRunning(), "should not be running initially")

	bsm.Start(true, 14, 0)
	assert.True(t, bsm.isRunning(), "should be running after start")

	bsm.Stop()
	assert.False(t, bsm.isRunning(), "should not be running after stop")
}

func TestBackgroundSyncManager_CalculateNextRunTime_Future(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// Schedule for a fixed hour:minute that is in the future
	now := time.Now()
	scheduleHour := now.Hour()
	scheduleMinute := now.Minute() + 5
	if scheduleMinute >= 60 {
		scheduleMinute -= 60
		scheduleHour = (scheduleHour + 1) % 24
	}

	nextRun := bsm.calculateNextRunTime(scheduleHour, scheduleMinute)

	assert.True(t, nextRun.After(now), "next run should be in the future")
	assert.Equal(t, scheduleHour, nextRun.Hour())
	assert.Equal(t, scheduleMinute, nextRun.Minute())
}

func TestBackgroundSyncManager_CalculateNextRunTime_Past(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// Schedule for a time that is definitely in the past (2 hours ago)
	now := time.Now()
	scheduleHour := (now.Hour() + 22) % 24 // 2 hours ago, safely wrapped
	scheduleMinute := now.Minute()

	nextRun := bsm.calculateNextRunTime(scheduleHour, scheduleMinute)

	// Should be tomorrow at the scheduled time
	assert.True(t, nextRun.After(now), "next run should be in the future (tomorrow)")
	assert.Equal(t, scheduleHour, nextRun.Hour())
	assert.Equal(t, scheduleMinute, nextRun.Minute())
}

func TestBackgroundSyncManager_SyncFolder_NewFiles(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so syncFolder doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// Sync folder with empty DB
	newCount, updatedCount, deletedCount, thumbCount := bsm.syncFolder(tmpDir, false)

	assert.Equal(t, 3, newCount, "should add 3 new files")
	assert.Equal(t, 0, updatedCount)
	assert.Equal(t, 0, deletedCount)
	assert.Equal(t, 0, thumbCount) // Thumbnails disabled
}

func TestBackgroundSyncManager_SyncFolder_Unchanged(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so syncFolder doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// First sync to populate DB
	bsm.syncFolder(tmpDir, false)

	// Second sync - all should be unchanged
	newCount, updatedCount, _, _ := bsm.syncFolder(tmpDir, false)

	assert.Equal(t, 0, newCount)
	assert.Equal(t, 0, updatedCount, "no files should be updated")
}

func TestBackgroundSyncManager_SyncFolder_DeletedFiles(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so syncFolder doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 5)

	// First sync to populate DB
	bsm.syncFolder(tmpDir, false)

	// Delete 2 files from disk
	require.GreaterOrEqual(t, len(paths), 2)
	fixtures.DeleteTestFile(t, paths[0])
	fixtures.DeleteTestFile(t, paths[1])

	// Cleanup should remove them
	deletedCount := bsm.cleanupMissingFiles()

	assert.Equal(t, 2, deletedCount, "should delete 2 missing file records")
}

func TestBackgroundSyncManager_InvalidateTagsAndEmbeddings(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Create an ImageFile
	img := testutil.SeedImageFile(t, bsm.db, "/tmp/test/invalidate.jpg", "abc123", 1024)

	// Seed tags and embeddings
	bsm.db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "landscape"})
	bsm.db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "nature"})
	bsm.db.Create(&domain.TagEmbedding{ImageFileID: img.ID, TagCount: 2})

	// Verify they exist
	var tagCountBefore, embCountBefore int64
	bsm.db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCountBefore)
	bsm.db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCountBefore)
	require.Equal(t, int64(2), tagCountBefore)
	require.Equal(t, int64(1), embCountBefore)

	// Invalidate
	bsm.invalidateTagsAndEmbeddings(img.ID)

	// Verify deleted
	var tagCountAfter, embCountAfter int64
	bsm.db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCountAfter)
	bsm.db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCountAfter)
	assert.Equal(t, int64(0), tagCountAfter, "tags should be invalidated")
	assert.Equal(t, int64(0), embCountAfter, "embeddings should be invalidated")
}

func TestBackgroundSyncManager_CleanupMissingFiles_CascadeDeletesChildren(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so cleanupMissingFiles doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	// Create an ImageFile with a non-existent path and seed children
	img := testutil.SeedImageFile(t, bsm.db, "/tmp/nonexistent/deleted.jpg", "hash", 512)
	bsm.db.Create(&domain.ImageMetadata{ImageFileID: img.ID, Width: 100, Height: 100})
	bsm.db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "tag1"})
	bsm.db.Create(&domain.TagEmbedding{ImageFileID: img.ID, TagCount: 1})

	// Run cleanup
	deleted := bsm.cleanupMissingFiles()
	assert.Equal(t, 1, deleted)

	// Verify parent and all children are gone
	var imgCount, metaCount, tagCount, embCount int64
	bsm.db.Model(&domain.ImageFile{}).Where("id = ?", img.ID).Count(&imgCount)
	bsm.db.Model(&domain.ImageMetadata{}).Where("image_file_id = ?", img.ID).Count(&metaCount)
	bsm.db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCount)
	bsm.db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCount)
	assert.Equal(t, int64(0), imgCount, "ImageFile should be deleted")
	assert.Equal(t, int64(0), metaCount, "ImageMetadata should be deleted")
	assert.Equal(t, int64(0), tagCount, "ImageTag should be deleted")
	assert.Equal(t, int64(0), embCount, "TagEmbedding should be deleted")
}
