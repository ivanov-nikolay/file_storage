package main

import (
	"context"

	"github.com/ivanov-nikolay/file_storage/internal/config"
	"github.com/ivanov-nikolay/file_storage/internal/handler"
	"github.com/ivanov-nikolay/file_storage/internal/storage"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.ResetIPStats(ctx)

	storage.InitRedis()
	config.InitStorage()
	handler.StartServer()
}
