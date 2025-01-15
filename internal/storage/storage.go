package storage

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/ivanov-nikolay/file_storage/internal/config"
)

// SaveFile сохраняет файл на диск
func SaveFile(hash string, file io.Reader) error {
	dir := filepath.Join(config.BaseStoragePath, hash[:2])

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	filePath := filepath.Join(dir, hash)
	newFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer newFile.Close()

	if _, err := io.Copy(newFile, file); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	log.Printf("File saved successfully: %s", filePath)
	return nil
}
