package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	observability "github.com/Gen-Do/lib-observability"
	platform "github.com/Gen-Do/lib-platform"
	"github.com/Gen-Do/lib-transport/listener"
	"github.com/go-chi/chi/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stream-gpt/service-configs/internal/api/config_batch"
	"github.com/stream-gpt/service-configs/internal/api/config_crud"
	"github.com/stream-gpt/service-configs/internal/bootwait"
	"github.com/stream-gpt/service-configs/internal/generated/server/api"
	"github.com/stream-gpt/service-configs/internal/metrics"
	"github.com/stream-gpt/service-configs/internal/migrate"
	"github.com/stream-gpt/service-configs/internal/repository"
	"github.com/stream-gpt/service-configs/internal/service"
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

	// sql.Open only validates the DSN — it does not establish a connection.
	// On a cold host reboot, docker compose / k8s start every container at
	// roughly the same time, so postgres may not be accepting connections
	// yet the first time we actually touch it (previously: migrate.Run
	// below would hard-fail here, and this service restarted ~19x during
	// the 2026-07-19 reboot before postgres came up). Ping with bounded
	// retry-with-backoff instead of fatal-exiting on the first attempt —
	// see internal/bootwait. A postgres still unreachable after
	// bootWaitCeiling (env BOOT_WAIT_CEILING, default 3m) is treated as
	// genuinely dead and still fatal-exits.
	pingErr := bootwait.WaitFor(ctx, log, "postgres", func(waitCtx context.Context) error {
		return db.PingContext(waitCtx)
	}, bootwait.Options{Ceiling: bootWaitCeiling()})
	if pingErr != nil {
		log.Error(log.WithError(ctx, pingErr), "Failed to connect to database")
		return platform.ExitCodeFailure
	}

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

// bootWaitCeiling returns the total time budget for boot-time dependency
// waits (see internal/bootwait), env-tunable via BOOT_WAIT_CEILING. Falls
// back to bootwait's own default (3m) if unset or unparsable.
func bootWaitCeiling() time.Duration {
	v := os.Getenv("BOOT_WAIT_CEILING")
	if v == "" {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0
	}
	return d
}
