package config

import (
	"log"
	"os"
)

const (
	BaseStoragePath = "./store" // Корневая директория хранилища файлов
	MaxUploadSize   = 10 << 20  // Максимальный объем загружаемого файла: 10 MB
	MaxStorageSize  = 10 << 30  // Максимальный объем хранилища файлов: 10 GB
)

const (
	// MaxConnectionsPerIP устанавливает ограничение на максимальное количество одновременных соединений
	//с API от одного IP-адреса
	MaxConnectionsPerIP = 10
	// MaxUploadBytesPerIP определяет максимальный объем данных, который может быть загружен на сервер
	//с одного IP-адреса в течение определенного периода (например, дня)
	MaxUploadBytesPerIP = 50 << 20 // 50 MB
	// MaxDownloadBytesPerIP определяет максимальный объем данных, который может быть скачан с сервера
	//одним IP-адресом за определенный период
	MaxDownloadBytesPerIP = 100 << 20 // 100 MB
	// MaxDeleteBytesPerIP устанавливает ограничение на общий объем данных, который может быть удален
	//с сервера одним IP-адресом за период
	MaxDeleteBytesPerIP = 10 << 20 // 10 MB
	// RequestsPerSecond ограничивает количество запросов, которые могут быть отправлены с одного IP-адреса в секунду
	RequestsPerSecond = 5 // 5 RPS
)

const (
	MaxDownloadSize = 1 << 10 // 1024
	DeleteSize      = 1 << 9  // 512
)

// InitStorage создает корневую директорию для хранения файлов
func InitStorage() {
	if err := os.MkdirAll(BaseStoragePath, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}
}
