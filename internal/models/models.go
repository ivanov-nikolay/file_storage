package models

import (
	"sync"
	"time"
)

// FileMetadata информавция о файле
type FileMetadata struct {
	FileName      string    `json:"file_name"`
	Size          int64     `json:"size"`
	UploadedAt    time.Time `json:"uploaded_at"`
	DownloadCount int       `json:"download_cnt"`
	Deleted       bool      `json:"deleted"`
}

// MetadataStore служит для хранения метаданных файлов
var MetadataStore = sync.Map{}
