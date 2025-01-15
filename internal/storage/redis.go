package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/ivanov-nikolay/file_storage/internal/config"
	"github.com/ivanov-nikolay/file_storage/internal/models"
	"github.com/redis/go-redis/v9"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
)

// InitRedis настраивает конфигурацию Redis
func InitRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Адрес вашего Redis-сервера
		Password: "",               // Пароль, если установлен
		DB:       0,                // Номер базы
	})

	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Starting Redis on port: 6379")
}

// SaveMetadataToRedis сохраняет метаданные о файле в БД Redis
func SaveMetadataToRedis(hash string, metadata models.FileMetadata) error {
	key := fmt.Sprintf("file:%s", hash)
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return redisClient.HSet(ctx, key, "metadata", data).Err()
}

// IncrementDownLoadCount увеличивает счетчик скачивания файла из БД Redis
func IncrementDownLoadCount(hash string) error {
	key := fmt.Sprintf("file:%s", hash)

	return redisClient.HIncrBy(ctx, key, "download_cnt", 1).Err()
}

// MarkFileAsDeleted помечает удаленный файл
func MarkFileAsDeleted(hash string) error {
	key := fmt.Sprintf("file:%s", hash)

	return redisClient.HSet(ctx, key, "deleted", true).Err()
}

// CleanupStorage определеяет и удаляет редко запрашиваемые файлы
func CleanupStorage(maxSize int64) (bool, error) {
	var totalSize int64
	files := make([]struct {
		Hash       string
		Size       int64
		UsageCount int
	}, 0)

	models.MetadataStore.Range(func(key, value interface{}) bool {
		hash := key.(string)
		metadata := value.(models.FileMetadata)
		totalSize += metadata.Size
		files = append(files, struct {
			Hash       string
			Size       int64
			UsageCount int
		}{
			Hash:       hash,
			Size:       metadata.Size,
			UsageCount: metadata.DownloadCount,
		})
		return true
	})

	if totalSize <= maxSize {
		return true, nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].UsageCount < files[j].UsageCount
	})

	for _, file := range files {
		if totalSize <= maxSize {
			break
		}
		filePath := filepath.Join(config.BaseStoragePath, file.Hash[0:2], file.Hash)
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to delete file %s: %v", file.Hash, err)
			continue
		}

		models.MetadataStore.Delete(file.Hash)

		if err := MarkFileAsDeleted(file.Hash); err != nil {
			log.Printf("Failed to mark file as deleted in Redis: %v", err)
		}

		totalSize -= file.Size

		log.Printf("Deleted file %s to free up space", file.Hash)
	}

	if totalSize > maxSize {
		return false, fmt.Errorf("failed to free up enough space, current size: %d, limit: %d", totalSize, maxSize)
	}
	return true, nil
}
