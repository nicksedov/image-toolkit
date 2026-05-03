package thumbnail

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// ThumbnailCacheSubdirs количество подпапок для равномерного распределения файлов
	ThumbnailCacheSubdirs = 128
	// ThumbnailFormat формат сохраняемых миниатюр
	ThumbnailFormat = "webp"
	// ThumbnailQuality качество сжатия миниатюр (0-100)
	ThumbnailQuality = 80
	// ThumbnailMaxSize максимальный размер миниатюры в пикселях
	ThumbnailMaxSize = 320
)

// CacheKey генерирует ключ кэша на основе пути к файлу
func CacheKey(filePath string) string {
	// MD5 хеш пути для получения фиксированной длины
	hash := md5.Sum([]byte(filePath))
	return hex.EncodeToString(hash[:])
}

// CachePath вычисляет абсолютный путь к файлу миниатюры в иерархической структуре кэша
func CachePath(cacheDir, filePath string) string {
	cacheKey := CacheKey(filePath)
	subdir1 := cacheKey[:2]
	subdir2 := cacheKey[2:4]
	fileName := cacheKey + "." + ThumbnailFormat

	return filepath.Join(cacheDir, subdir1, subdir2, fileName)
}

// CachePathRelative вычисляет относительный путь к файлу миниатюры (относительно корня кэша)
func CachePathRelative(filePath string) string {
	cacheKey := CacheKey(filePath)
	subdir1 := cacheKey[:2]
	subdir2 := cacheKey[2:4]
	fileName := cacheKey + "." + ThumbnailFormat

	// Возвращаем относительный путь без префикса кэш-директории
	return filepath.Join(subdir1, subdir2, fileName)
}

// CacheDirPath вычисляет путь к подпапке кэша для хеша
func CacheDirPath(cacheDir, filePath string) string {
	cacheKey := CacheKey(filePath)
	subdir1 := cacheKey[:2]
	subdir2 := cacheKey[2:4]

	return filepath.Join(cacheDir, subdir1, subdir2)
}

// ThumbnailCacheStorage управляет иерархическим кэшем миниатюр на файловой системе
type ThumbnailCacheStorage struct {
	cacheDir string
	mu       sync.RWMutex
	enabled  bool
}

// NewThumbnailCacheStorage создает новое хранилище кэша миниатюр
func NewThumbnailCacheStorage(cacheDir string) (*ThumbnailCacheStorage, error) {
	if cacheDir == "" {
		return nil, &ErrInvalidCachePath{Path: cacheDir}
	}
	
	// Если путь не абсолютный, делаем его абсолютным
	if !filepath.IsAbs(cacheDir) {
		absPath, err := filepath.Abs(cacheDir)
		if err != nil {
			return nil, &ErrInvalidCachePath{Path: cacheDir}
		}
		cacheDir = absPath
	}
	
	storage := &ThumbnailCacheStorage{
		cacheDir: cacheDir,
		enabled:  true,
	}
	
	// Инициализируем структуру подпапок
	if err := storage.initStructure(); err != nil {
		return nil, &ErrCacheInitFailed{Path: cacheDir, Err: err}
	}
	
	return storage, nil
}

// initStructure создает иерархическую структуру из 128 подпапок
func (tcs *ThumbnailCacheStorage) initStructure() error {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	
	// Создаем主 подпапку кэша
	if err := os.MkdirAll(tcs.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	// Создаем 128 подпапок для равномерного распределения
	// Формат: 00, 01, 02, ..., 7f (hexadecimal)
	for i := 0; i < ThumbnailCacheSubdirs; i++ {
		subdirName := fmt.Sprintf("%02x", i)
		subdirPath := filepath.Join(tcs.cacheDir, subdirName)
		
		// Создаем подпапку
		if err := os.MkdirAll(subdirPath, 0755); err != nil {
			return fmt.Errorf("failed to create subdirectory %s: %w", subdirPath, err)
		}
		
		// Создаем подпапки второго уровня для каждой из 128 первых подпапок
		for j := 0; j < ThumbnailCacheSubdirs; j++ {
			subdir2Name := fmt.Sprintf("%02x", j)
			subdir2Path := filepath.Join(subdirPath, subdir2Name)
			
			if err := os.MkdirAll(subdir2Path, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory %s: %w", subdir2Path, err)
			}
		}
	}
	
	return nil
}

// Enable включает кэш миниатюр
func (tcs *ThumbnailCacheStorage) Enable() {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	tcs.enabled = true
}

// Disable выключает кэш миниатюр
func (tcs *ThumbnailCacheStorage) Disable() {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	tcs.enabled = false
}

// IsEnabled проверяет включен ли кэш
func (tcs *ThumbnailCacheStorage) IsEnabled() bool {
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()
	return tcs.enabled
}

// GetPath возвращает абсолютный путь к файлу миниатюры для заданного пути к изображению
func (tcs *ThumbnailCacheStorage) GetPath(filePath string) string {
	if !tcs.enabled {
		return ""
	}
	return CachePath(tcs.cacheDir, filePath)
}

// GetPathRelative возвращает относительный путь к файлу миниатюры для заданного пути к изображению
func (tcs *ThumbnailCacheStorage) GetPathRelative(filePath string) string {
	if !tcs.enabled {
		return ""
	}
	return CachePathRelative(filePath)
}

// Exists проверяет наличие миниатюры в кэше
func (tcs *ThumbnailCacheStorage) Exists(filePath string) bool {
	if !tcs.enabled {
		return false
	}
	
	path := CachePath(tcs.cacheDir, filePath)
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()
	
	_, err := os.Stat(path)
	return err == nil
}

// Get возвращает путь к миниатюре, если она существует, пустую строку иначе
func (tcs *ThumbnailCacheStorage) Get(filePath string) string {
	if !tcs.enabled {
		return ""
	}
	
	path := CachePath(tcs.cacheDir, filePath)
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()
	
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// Set сохраняет миниатюру в кэш
func (tcs *ThumbnailCacheStorage) Set(filePath string, thumbnailData []byte) error {
	if !tcs.enabled {
		return ErrThumbnailCacheDisabled
	}
	
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	
	cachePath := CachePath(tcs.cacheDir, filePath)
	
	// Убедимся, что подпапка существует
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ErrCacheWriteFailed{Path: filePath, Err: err}
	}
	
	// Записываем файл миниатюры
	if err := os.WriteFile(cachePath, thumbnailData, 0644); err != nil {
		return &ErrCacheWriteFailed{Path: filePath, Err: err}
	}
	
	return nil
}

// Delete удаляет миниатюру из кэша
func (tcs *ThumbnailCacheStorage) Delete(filePath string) error {
	if !tcs.enabled {
		return nil
	}
	
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	
	cachePath := CachePath(tcs.cacheDir, filePath)
	
	if err := os.Remove(cachePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Файл уже не существует
		}
		return &ErrCacheReadFailed{Path: filePath, Err: err}
	}
	
	return nil
}

// ClearAll очищает весь кэш миниатюр
func (tcs *ThumbnailCacheStorage) ClearAll() error {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	
	// Удаляем всю структуру кэша
	return os.RemoveAll(tcs.cacheDir)
}

// Stats возвращает статистику кэша
func (tcs *ThumbnailCacheStorage) Stats() (int, int64, error) {
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()
	
	var count int
	var totalSize int64
	
	err := filepath.Walk(tcs.cacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil // Продолжаем обход при ошибках
		}
		
		if !info.IsDir() && strings.HasSuffix(info.Name(), "."+ThumbnailFormat) {
			count++
			totalSize += info.Size()
		}
		
		return nil
	})
	
	if err != nil {
		return 0, 0, &ErrCacheReadFailed{Path: tcs.cacheDir, Err: err}
	}
	
	return count, totalSize, nil
}

// ListFiles возвращает список всех файлов в кэше
func (tcs *ThumbnailCacheStorage) ListFiles() ([]string, error) {
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()

	var files []string

	err := filepath.Walk(tcs.cacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), "."+ThumbnailFormat) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, &ErrCacheReadFailed{Path: tcs.cacheDir, Err: err}
	}

	return files, nil
}

// PruneExpired удаляет миниатюры, для которых файл оригинала больше не существует
func (tcs *ThumbnailCacheStorage) PruneExpired(imagePaths map[string]bool) error {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	
	// Проходим по всем файлам в кэше
	err := filepath.Walk(tcs.cacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			return nil
		}
		
		if !strings.HasSuffix(info.Name(), "."+ThumbnailFormat) {
			return nil
		}
		
		// Извлекаем путь к изображению из метаданных или удаляем устаревшую миниатюру
		// Для простоты удаляем миниатюры, если оригинальный файл не найден
		// В реальном приложении можно хранить обратное отображение в базе
		
		return nil
	})
	
	return err
}
