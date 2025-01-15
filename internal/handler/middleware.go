package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/ivanov-nikolay/file_storage/internal/config"
)

// IPStats структура для трекинга лимитов
type IPStats struct {
	Connections   int64         // Одновременные соединения
	UploadBytes   int64         // Байты загруженных данных
	DownloadBytes int64         // Байты скачанных данных
	DeleteBytes   int64         // Байты удаленных данных
	RateLimiter   *rate.Limiter // Лимит запросов (RPS)
	mu            sync.Mutex    // Для атомарных операций
}

// ipStats хранилище для статистики IP
var ipStats sync.Map

// limitMiddleware ...
func limitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		stats, _ := ipStats.LoadOrStore(ip, &IPStats{
			RateLimiter: rate.NewLimiter(config.RequestsPerSecond, config.RequestsPerSecond),
		})
		ipStat := stats.(*IPStats)

		// Ограничение RPS
		if !ipStat.RateLimiter.Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		// Ограничение одновременных соединений
		ipStat.mu.Lock()
		if ipStat.Connections >= config.MaxConnectionsPerIP {
			http.Error(w, "too many connections from this IP", http.StatusTooManyRequests)
			return
		}
		ipStat.Connections++
		ipStat.mu.Unlock()

		// Снимаем соединение после завершения обработки
		defer func() {
			ipStat.mu.Lock()
			ipStat.Connections--
			ipStat.mu.Unlock()
		}()

		// Проверка лимитов на байты
		if r.Method == http.MethodPost && r.ContentLength > 0 {
			if ipStat.UploadBytes+r.ContentLength > config.MaxUploadBytesPerIP {
				http.Error(w, "upload limit exceeded", http.StatusForbidden)
				return
			}
			ipStat.UploadBytes += r.ContentLength
		} else if r.Method == http.MethodGet {
			if ipStat.DownloadBytes+int64(config.MaxDownloadSize) > config.MaxDownloadBytesPerIP {
				http.Error(w, "download limit exceeded", http.StatusForbidden)
				return
			}
			ipStat.DownloadBytes += int64(config.MaxDownloadSize)
		} else if r.Method == http.MethodDelete {
			if ipStat.DeleteBytes+int64(config.DeleteSize) > config.MaxDeleteBytesPerIP {
				http.Error(w, "delete limit exceeded", http.StatusForbidden)
				return
			}
			ipStat.DeleteBytes += int64(config.DeleteSize)
		}

		next.ServeHTTP(w, r)
	})
}

// ResetIPStats cбрасывает статистику
func ResetIPStats(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
			ipStats.Range(func(key, value interface{}) bool {
				stats := value.(*IPStats)
				stats.mu.Lock()
				stats.UploadBytes = 0
				stats.DownloadBytes = 0
				stats.DeleteBytes = 0
				stats.mu.Unlock()
				return true
			})
		}
	}
}
