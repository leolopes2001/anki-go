package application

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	service *Service
	logger  *slog.Logger
}

func NewWorker(service *Service, logger *slog.Logger) *Worker {
	return &Worker{service: service, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed, err := w.service.ProcessNext(ctx)
			if err != nil {
				w.logger.Error("queue processing failed", "operation", "process_queue", "error", err)
			}
			if processed {
				w.logger.Info("queue item processed", "operation", "process_queue")
			}
		}
	}
}
