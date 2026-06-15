package imaging

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"image-toolkit/internal/domain"
	"image-toolkit/internal/testutil"
	"image-toolkit/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventCollector is a thread-safe collector for FileEvents used in tests.
type eventCollector struct {
	mu     sync.Mutex
	events []FileEvent
}

func (ec *eventCollector) add(event FileEvent) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.events = append(ec.events, event)
}

func (ec *eventCollector) get() []FileEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	result := make([]FileEvent, len(ec.events))
	copy(result, ec.events)
	return result
}

func (ec *eventCollector) count() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.events)
}

// waitForEvents polls until the collector has at least n events or times out.
func (ec *eventCollector) waitForEvents(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ec.count() >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ec.count() >= n
}

func TestPostProcessor_ScanDirectory_EmitsCreatedEvents(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	collector := &eventCollector{}
	sem := make(chan struct{}, 4)

	progressChan := make(chan string, 100)
	err := scanDirectory(db, tmpDir, progressChan, 2, collector.add, sem)
	require.NoError(t, err)

	require.True(t, collector.waitForEvents(3, 2*time.Second), "should receive 3 created events")

	events := collector.get()
	assert.Len(t, events, 3)
	for _, e := range events {
		assert.Equal(t, FileCreated, e.Type)
		assert.NotZero(t, e.ImageFileID)
		assert.NotEmpty(t, e.Path)
	}
}

func TestPostProcessor_ScanDirectory_EmitsModifiedEvents(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)

	// First scan without callback to populate DB
	progressChan := make(chan string, 100)
	err := scanDirectory(db, tmpDir, progressChan, 1, nil, nil)
	require.NoError(t, err)

	// Modify the file with different dimensions (different content)
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 200, 200)

	// Second scan with callback
	collector := &eventCollector{}
	sem := make(chan struct{}, 4)
	progressChan2 := make(chan string, 100)
	err = scanDirectory(db, tmpDir, progressChan2, 1, collector.add, sem)
	require.NoError(t, err)

	require.True(t, collector.waitForEvents(1, 2*time.Second), "should receive 1 modified event")

	events := collector.get()
	require.Len(t, events, 1)
	assert.Equal(t, FileModified, events[0].Type)
	assert.True(t, events[0].ContentChanged, "content should be detected as changed (different hash)")
	assert.NotZero(t, events[0].ImageFileID)
}

func TestPostProcessor_FastScan_EmitsDeletedEvents(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 5)

	// First scan without callback
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1, nil, nil)

	// Delete 2 files from disk
	os.Remove(paths[0])
	os.Remove(paths[1])

	// Fast scan with callback
	collector := &eventCollector{}
	sem := make(chan struct{}, 4)
	progressChan2 := make(chan string, 100)
	result := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1, collector.add, sem)

	assert.Equal(t, 2, result.Deleted)

	require.True(t, collector.waitForEvents(2, 2*time.Second), "should receive 2 deleted events")

	events := collector.get()
	assert.Len(t, events, 2)
	for _, e := range events {
		assert.Equal(t, FileDeleted, e.Type)
		assert.NotZero(t, e.ImageFileID)
		assert.NotEmpty(t, e.Path)
	}
}

func TestPostProcessor_CleanupMissingFiles_EmitsDeletedEvents(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// First scan without callback
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1, nil, nil)

	// Delete 1 file from disk
	os.Remove(paths[0])

	// Cleanup with callback
	collector := &eventCollector{}
	sem := make(chan struct{}, 4)
	progressChan2 := make(chan string, 100)
	err := cleanupMissingFiles(db, progressChan2, collector.add, sem)
	require.NoError(t, err)

	require.True(t, collector.waitForEvents(1, 2*time.Second), "should receive 1 deleted event")

	events := collector.get()
	require.Len(t, events, 1)
	assert.Equal(t, FileDeleted, events[0].Type)
	assert.NotZero(t, events[0].ImageFileID)
}

func TestPostProcessor_NilCallback_NoError(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	progressChan := make(chan string, 100)
	err := scanDirectory(db, tmpDir, progressChan, 2, nil, nil)
	require.NoError(t, err)

	var count int64
	db.Model(&domain.ImageFile{}).Count(&count)
	assert.Equal(t, int64(3), count, "scan should work normally without callback")
}

func TestPostProcessor_BoundedConcurrency(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	const fileCount = 20
	for i := 0; i < fileCount; i++ {
		fixtures.CreateMinimalJPEG(t, tmpDir, fmt.Sprintf("img_%02d.jpg", i), 100, 100)
	}

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var wg sync.WaitGroup

	sem := make(chan struct{}, 2) // bound to 2 concurrent goroutines

	callback := func(event FileEvent) {
		cur := currentConcurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		// Yield to give other goroutines a chance to run concurrently
		time.Sleep(5 * time.Millisecond)
		currentConcurrent.Add(-1)
		wg.Done()
	}

	wg.Add(fileCount)

	progressChan := make(chan string, 200)
	err := scanDirectory(db, tmpDir, progressChan, 4, callback, sem)
	require.NoError(t, err)

	wg.Wait()

	observed := maxConcurrent.Load()
	assert.LessOrEqual(t, int(observed), 2, "max concurrent goroutines should not exceed semaphore capacity of 2")
	assert.GreaterOrEqual(t, int(observed), 1, "at least 1 goroutine should have run")
}

func TestPostProcessor_FastScan_EmitsCreatedAndModifiedEvents(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "existing.jpg", 100, 100)

	// First scan without callback
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1, nil, nil)

	// Add a new file and modify existing one
	fixtures.CreateMinimalJPEG(t, tmpDir, "new.jpg", 150, 150)
	fixtures.CreateMinimalJPEG(t, tmpDir, "existing.jpg", 200, 200)

	// Fast scan with callback
	collector := &eventCollector{}
	sem := make(chan struct{}, 4)
	progressChan2 := make(chan string, 100)
	result := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1, collector.add, sem)

	assert.GreaterOrEqual(t, result.Created, 1)
	assert.GreaterOrEqual(t, result.Modified, 1)

	require.True(t, collector.waitForEvents(2, 2*time.Second), "should receive at least 2 events")

	events := collector.get()
	var created, modified int
	for _, e := range events {
		switch e.Type {
		case FileCreated:
			created++
		case FileModified:
			modified++
		}
	}
	assert.GreaterOrEqual(t, created, 1, "should have at least 1 created event")
	assert.GreaterOrEqual(t, modified, 1, "should have at least 1 modified event")
}
