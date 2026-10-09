package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/local/anki-vocabulary/apps/api/internal/application"
	"github.com/local/anki-vocabulary/apps/api/internal/infrastructure/httpapi"
	"github.com/local/anki-vocabulary/apps/api/internal/infrastructure/provider"
	"github.com/local/anki-vocabulary/apps/api/internal/infrastructure/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	dataDir := environment("DATA_DIR", "./data")
	storageDir := environment("STORAGE_DIR", "./storage")
	repository, err := storage.NewJSONRepository(dataDir)
	if err != nil {
		logger.Error("initialize repository", "error", err)
		os.Exit(1)
	}
	media, err := storage.NewMediaStore(storageDir)
	if err != nil {
		logger.Error("initialize media store", "error", err)
		os.Exit(1)
	}
	providers := provider.New(
		logger,
		environment("DEEPSEEK_URL", "https://api.thehive.ai/api/v3"), os.Getenv("DEEPSEEK_API_KEY"), environment("DEEPSEEK_MODEL", "deepseek-ai/deepseek-v4.1-flash"),
		os.Getenv("ELEVENLABS_API_KEY"), os.Getenv("ELEVENLABS_VOICE_ID"),
		environment("ANKI_CONNECT_URL", "http://localhost:8765"), storageDir,
		os.Getenv("PIXABAY_API_KEY"),
	)
	service := application.NewService(repository, providers, providers, providers, providers, providers, media)
	handler := httpapi.New(service, logger)

	server := &http.Server{
		Addr:              ":" + environment("API_PORT", "8080"),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workers, _ := strconv.Atoi(environment("QUEUE_WORKERS", "1"))
	for range max(workers, 1) {
		go application.NewWorker(service, logger).Run(ctx)
	}
	go func() {
		logger.Info("api listening", "port", environment("API_PORT", "8080"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api shutdown failed", "error", err)
		os.Exit(1)
	}
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
