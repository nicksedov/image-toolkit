package imaging

import (
	"testing"
	"time"

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
	assert.True(t, bsm.IsRunning(), "sync should be running")

	bsm.Stop()
}

func TestBackgroundSyncManager_Stop(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	bsm.Start(true, 14, 0)
	bsm.Stop()

	time.Sleep(50 * time.Millisecond)
	assert.False(t, bsm.IsRunning(), "sync should not be running")
}

func TestBackgroundSyncManager_IsRunning(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	assert.False(t, bsm.IsRunning(), "should not be running initially")

	bsm.Start(true, 14, 0)
	assert.True(t, bsm.IsRunning(), "should be running after start")

	bsm.Stop()
	assert.False(t, bsm.IsRunning(), "should not be running after stop")
}

func TestBackgroundSyncManager_CalculateNextRunTime_Future(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// Use fixed times to avoid minute overflow issues
	now := time.Now()
	scheduleHour := now.Hour()
	scheduleMinute := 30 // Fixed minute

	// If 30 minutes is in the past, use hour+1
	if scheduleMinute <= now.Minute() {
		scheduleHour = (now.Hour() + 1) % 24
	}

	nextRun := bsm.calculateNextRunTime(scheduleHour, scheduleMinute)

	assert.True(t, nextRun.After(now), "next run should be in the future")
	assert.Equal(t, scheduleHour, nextRun.Hour())
	assert.Equal(t, scheduleMinute, nextRun.Minute())
}

func TestBackgroundSyncManager_CalculateNextRunTime_Past(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// Schedule time is in the past (1 minute ago)
	now := time.Now()
	scheduleHour := now.Hour()
	scheduleMinute := now.Minute() - 1

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
