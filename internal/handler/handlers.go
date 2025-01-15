package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/ivanov-nikolay/file_storage/internal/config"
	"github.com/ivanov-nikolay/file_storage/internal/models"
	"github.com/ivanov-nikolay/file_storage/internal/storage"
	"github.com/ivanov-nikolay/file_storage/internal/utils"
)

var (
	uploadSemaphore = semaphore.NewWeighted(10) // Максимум 10 запросов
	fileAccess      sync.Mutex                  // Мьютекс для защиты общего ресурса
)

type PreUploadCallback func(r *http.Request) error
type PostUploadCallback func(hash string, metadata models.FileMetadata) error

var (
	preUploadCallback  PreUploadCallback
	postUploadCallback PostUploadCallback
)

var (
	preDownloadCallback  func(hash string, file *os.File) error
	postDownloadCallback func(hash string)
)

// handleUpload обрабатывает запрос на загрузку файла.
func handleUpload(w http.ResponseWriter, r *http.Request) {
	ok, err := storage.CleanupStorage(config.MaxStorageSize)
	if err != nil {
		http.Error(w, "failed to clean up storage: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "failed to free up enough space", http.StatusInternalServerError)
		return
	}

	if !uploadSemaphore.TryAcquire(1) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	defer uploadSemaphore.Release(1)

	if preUploadCallback != nil {
		if err := preUploadCallback(r); err != nil {
			http.Error(w, "pre-upload callback failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, config.MaxUploadSize)

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "invalid file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Копируем файл для повторного использования
	var fileCopy io.ReadSeeker
	fileBuffer := new(bytes.Buffer)
	if _, err := io.Copy(fileBuffer, file); err != nil {
		http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fileCopy = bytes.NewReader(fileBuffer.Bytes())

	hash, err := utils.HashFile(fileCopy)
	if err != nil {
		http.Error(w, "failed to calculate hash: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fileCopy.Seek(0, io.SeekStart)

	log.Printf("Uploaded file: %s", header.Filename)
	log.Printf("Calculated hash: %s", hash)

	if err := storage.SaveFile(hash, fileCopy); err != nil {
		http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	metadata := models.FileMetadata{
		FileName:   header.Filename,
		Size:       header.Size,
		UploadedAt: time.Now(),
	}

	models.MetadataStore.Store(hash, metadata)
	log.Printf("Saving metadata to Redis: hash=%s, metadata=%+v", hash, metadata)
	if err := storage.SaveMetadataToRedis(hash, metadata); err != nil {
		log.Printf("Failed to save metadata to Redis: %v", err)
		http.Error(w, "failed to save metadata to Redis: "+err.Error(), http.StatusInternalServerError)
	}

	if postUploadCallback != nil {
		if err := postUploadCallback(hash, metadata); err != nil {
			http.Error(w, "post-upload callback failed: "+err.Error(), http.StatusInternalServerError)
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write([]byte(fmt.Sprintf("file saved with hash: %s", hash)))
}

// handleDownload обрабатывает запрос на скачивание файла.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	if !uploadSemaphore.TryAcquire(1) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	defer uploadSemaphore.Release(1)

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, "hash is required", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(config.BaseStoragePath, hash[0:2], hash)
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to open file", http.StatusInternalServerError)
		}
		return
	}
	defer file.Close()

	if preDownloadCallback != nil {
		if err := preDownloadCallback(hash, file); err != nil {
			log.Printf("pre-download callback failed: %v", err)
			http.Error(w, "pre-download callback failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if val, exists := models.MetadataStore.Load(hash); exists {
		fileAccess.Lock()
		data := val.(models.FileMetadata)
		data.DownloadCount++
		models.MetadataStore.Store(hash, data)
		fileAccess.Unlock()
	}

	if err := storage.IncrementDownLoadCount(hash); err != nil {
		http.Error(w, "failed to increment download count: "+err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)

	if postDownloadCallback != nil {
		postDownloadCallback(hash)
	}
}

// handleDelete обрабатывает запрос на удаление файла.
func handleDelete(w http.ResponseWriter, r *http.Request) {
	if !uploadSemaphore.TryAcquire(1) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	defer uploadSemaphore.Release(1)

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, "hash is required", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(config.BaseStoragePath, hash[0:2], hash)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to delete file: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	models.MetadataStore.Delete(hash)

	if err := storage.MarkFileAsDeleted(hash); err != nil {
		http.Error(w, "failed to mark file as deleted: "+err.Error(), http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetupCallbacks настройка callback'ов
func SetupCallbacks() {
	preUploadCallback = func(r *http.Request) error {
		log.Println("Pre-upload callback: checking the request")

		file, _, err := r.FormFile("file")
		if err != nil {
			return err
		}
		defer file.Close()

		var fileBuffer bytes.Buffer
		if _, err := io.Copy(&fileBuffer, file); err != nil {
			return fmt.Errorf("failed to read file contents: %v", err)
		}

		md5Hash := r.FormValue("md5")
		sha1Hash := r.FormValue("sha1")
		sha256Hash := r.FormValue("sha256")

		if md5Hash != "" {
			calculatedMD5, err := utils.CalculateHash(bytes.NewReader(fileBuffer.Bytes()), "md5")
			if err != nil {
				return fmt.Errorf("error calculating MD5: %v", err)
			}
			if calculatedMD5 != md5Hash {
				return fmt.Errorf("MD5 hash does not match")
			}
		}

		if sha1Hash != "" {
			calculateSHA1, err := utils.CalculateHash(bytes.NewReader(fileBuffer.Bytes()), "sha1")
			if err != nil {
				return fmt.Errorf("failed to calculate SHA1: %v", err)
			}
			if calculateSHA1 != sha1Hash {
				return fmt.Errorf("SHA1 hash does not match")
			}
		}

		if sha256Hash != "" {
			calculateSHA256, err := utils.CalculateHash(bytes.NewReader(fileBuffer.Bytes()), "sha256")
			if err != nil {
				return fmt.Errorf("error calculating SHA256: %v", err)
			}
			if calculateSHA256 != sha256Hash {
				return fmt.Errorf("SHA256 hash does not match")
			}
		}

		log.Println("Pre-upload callback: hash verification completed successfully")
		return nil
	}

	postUploadCallback = func(hash string, metadata models.FileMetadata) error {
		log.Printf("Post-upload callback: file with hash %s successfully downloaded. Size: %d bytes", hash, metadata.Size)
		return nil
	}

	preDownloadCallback = func(hash string, file *os.File) error {

		calculatedHash, err := utils.HashFile(file)
		if err != nil {
			return fmt.Errorf("failed to calculate file hash: %v", err)
		}

		if calculatedHash != hash {
			return fmt.Errorf("integrity check failed: expected %s, got %s", hash, calculatedHash)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to reset file pointer: %v", err)
		}

		return nil
	}

	postDownloadCallback = func(hash string) {
		log.Printf("File with hash %s successfully downloaded", hash)
	}
}
