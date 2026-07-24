// Package main is the entry point for the Order Flow Engine background worker process.
package main

import (
	"context"
	"database/sql"
	"log"
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
	cfg := config.Load()
	log.Printf("Starting Order Flow Engine Worker (workers count: %d)...", cfg.WorkerCount)

	metrics.StartServer(":2113")

	db, err := sql.Open("pgx", cfg.DBConnString)
	if err != nil {
		log.Fatalf("Failed to open DB connection: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing DB: %v", err)
		}
	}()

	if err := db.Ping(); err != nil {
		log.Printf("Warning: Database ping failed (will retry on operations): %v", err)
	} else {
		log.Println("Connected to PostgreSQL successfully.")
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
		log.Fatalf("Failed to initialize RabbitMQ consumer: %v", err)
	}

	processor := worker.NewOrderProcessor(repo)
	pool := worker.NewPool(consumer, processor, cfg.WorkerCount)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	log.Println("Worker pool started and listening for incoming order tasks...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	log.Printf("Received shutdown signal (%v). Initiating graceful shutdown...", sig)

	if err := pool.Stop(); err != nil {
		log.Printf("Error stopping worker pool: %v", err)
	} else {
		log.Println("Worker pool stopped cleanly.")
	}

	log.Println("Worker service shutdown complete.")
}
