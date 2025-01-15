package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// HashFile возвращает SHA-256 хэш для файла
func HashFile(file io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateHash вычисляет hash файла
func CalculateHash(reader io.Reader, algorithm string) (string, error) {
	var hash io.Writer

	switch algorithm {
	case "md5":
		hash = md5.New()
	case "sha1":
		hash = sha1.New()
	case "sha256":
		hash = sha256.New()
	}

	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.(interface {
		Sum([]byte) []byte
	}).Sum(nil)), nil
}
