package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	observability "github.com/Gen-Do/lib-observability"
	platform "github.com/Gen-Do/lib-platform"
	"github.com/Gen-Do/lib-transport/listener"
	"github.com/stream-gpt/service-configs/internal/api/config_batch"
	"github.com/stream-gpt/service-configs/internal/api/config_crud"
	"github.com/stream-gpt/service-configs/internal/generated/server/api"
	"github.com/stream-gpt/service-configs/internal/metrics"
	"github.com/stream-gpt/service-configs/internal/migrate"
	"github.com/stream-gpt/service-configs/internal/repository"
	"github.com/stream-gpt/service-configs/internal/service"
	"github.com/go-chi/chi/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx := context.Background()

	obs := observability.MustNew(ctx)
	defer obs.Shutdown(ctx)

	log := obs.GetLogger()
	log.Info(ctx, "Initializing service")

	databaseDSN := os.Getenv("DATABASE_DSN")
	if databaseDSN == "" {
		databaseDSN = os.Getenv("DEP_DATABASE_DSN")
	}

	db, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		log.Error(log.WithError(ctx, err), "Failed to connect to database")
		return platform.ExitCodeFailure
	}
	defer db.Close()

	if err := migrate.Run(ctx, db); err != nil {
		log.Error(log.WithError(ctx, err), "Failed to run migrations")
		return platform.ExitCodeFailure
	}

	m := metrics.New(obs.GetMetrics().GetRegistry())

	repo := repository.NewPostgresConfigRepository(db, m)
	svc := service.NewConfigService(repo, m)

	crudHandler := config_crud.NewHandler(svc)
	batchHandler := config_batch.NewHandler(svc)

	srv := api.CreateHandler(
		api.WithMW(middleware.RequestID),
		api.WithMW(m.InFlightMiddleware()),
		api.WithMW(obs.HTTPMiddleware()),
		api.WithBaseURL("/v1"),
	)
	obs.RegisterRoutes(srv.GetMux())

	srv.SetCreateConfigHandler(crudHandler.Create)
	srv.SetListConfigsHandler(crudHandler.List)
	srv.SetGetConfigHandler(crudHandler.Get)
	srv.SetUpdateConfigHandler(crudHandler.Update)
	srv.SetDeleteConfigHandler(crudHandler.Delete)
	srv.SetBatchGetConfigsHandler(batchHandler.BatchGet)

	lis := listener.New(
		listener.WithIdleTimeout(10*time.Second),
		listener.WithReadTimeout(10*time.Second),
		listener.WithWriteTimeout(10*time.Second),
		listener.WithLogger(log),
	)

	err = platform.Run(ctx,
		platform.WithListener(lis),
		platform.WithMux(srv.GetMux()),
		platform.WithLogger(log),
		platform.WithEnableSignalHandling(true),
		platform.WithObservability(platform.ObservabilitySettings{
			Logger:  log,
			Metrics: nil,
		}),
	)
	if err != nil {
		log.Error(log.WithError(ctx, err), "Application exited with error")
		return platform.ExitCodeFailure
	}

	log.Info(ctx, "Service stopped gracefully")

	return platform.ExitCodeSuccess
}
