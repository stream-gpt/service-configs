package example

import (
	"context"
	"time"

	"github.com/Gen-Do/lib-observability/logger"
)

type Worker struct {
	logger logger.Logger
}

func NewWorker(logger logger.Logger) Worker {
	return Worker{logger: logger}
}

func (w Worker) Run(ctx context.Context) error {
	w.logger.Info(ctx, "Worker started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info(ctx, "Worker stopped")
			return ctx.Err()
		default:
			w.logger.Info(ctx, "Example worker tick")
			time.Sleep(1 * time.Second)
		}
	}
	// Если воркер завершит работу - весь контекст будет завершен
}
