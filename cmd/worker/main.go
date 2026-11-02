// Package main is the entry point for the Order Flow Engine background worker process.
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/arthkinq/order-flow-engine/internal/config"
	"github.com/arthkinq/order-flow-engine/internal/metrics"
	"github.com/arthkinq/order-flow-engine/internal/queue"
	"github.com/arthkinq/order-flow-engine/internal/repository"
	"github.com/arthkinq/order-flow-engine/internal/worker"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()
	slog.Info("starting Order Flow Engine Worker", slog.Int("worker_count", cfg.WorkerCount))

	metrics.StartServer(":2113")

	db, err := sql.Open("pgx", cfg.DBConnString)
	if err != nil {
		slog.Error("failed to open DB connection", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("error closing DB", slog.String("error", err.Error()))
		}
	}()

	if err := db.Ping(); err != nil {
		slog.Warn("database ping failed (will retry on operations)", slog.String("error", err.Error()))
	} else {
		slog.Info("connected to PostgreSQL successfully")
	}

	repo := repository.NewPostgresRepository(db)

	consumerCfg := queue.ConsumerConfig{
		URL:           cfg.RabbitMQURL,
		Queue:         cfg.Queue,
		ConsumerTag:   "order-worker-process",
		PrefetchCount: 10,
	}
	consumer, err := queue.NewRabbitMQConsumer(consumerCfg)
	if err != nil {
		slog.Error("failed to initialize RabbitMQ consumer", slog.String("error", err.Error()))
		os.Exit(1)
	}

	processor := worker.NewOrderProcessor(repo)
	pool := worker.NewPool(consumer, processor, cfg.WorkerCount)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		slog.Error("failed to start worker pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("worker pool started and listening for incoming order tasks")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	slog.Info("received shutdown signal, initiating graceful shutdown", slog.String("signal", sig.String()))

	if err := pool.Stop(); err != nil {
		slog.Error("error stopping worker pool", slog.String("error", err.Error()))
	} else {
		slog.Info("worker pool stopped cleanly")
	}

	slog.Info("worker service shutdown complete")
}
