// Package main is the entry point for the Order Flow Engine gRPC API server process.
package main

import (
	"database/sql"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/arthkinq/order-flow-engine/internal/config"
	"github.com/arthkinq/order-flow-engine/internal/metrics"
	"github.com/arthkinq/order-flow-engine/internal/queue"
	"github.com/arthkinq/order-flow-engine/internal/ratelimit"
	"github.com/arthkinq/order-flow-engine/internal/repository"
	"github.com/arthkinq/order-flow-engine/internal/server"
	orderv1 "github.com/arthkinq/order-flow-engine/proto/order/v1"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	log.Printf("Starting Order Flow Engine API Server on port %s...", cfg.GRPCServerPort)

	metrics.StartServer(":2112")

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
		log.Printf("Warning: Database ping failed: %v", err)
	} else {
		log.Println("Connected to PostgreSQL successfully.")
	}

	repo := repository.NewPostgresRepository(db)

	pubCfg := queue.SetupConfig{
		URL:        cfg.RabbitMQURL,
		Exchange:   cfg.Exchange,
		Queue:      cfg.Queue,
		RoutingKey: cfg.RoutingKey,
	}
	publisher, err := queue.NewRabbitMQPublisher(pubCfg)
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ publisher: %v", err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			log.Printf("Error closing publisher: %v", err)
		}
	}()

	limiter, err := ratelimit.NewGateKeeperClient(cfg.GateKeeperURL)
	if err != nil {
		log.Printf("Warning: Failed to create GateKeeper client: %v", err)
	} else {
		defer func() {
			if err := limiter.Close(); err != nil {
				log.Printf("Error closing limiter: %v", err)
			}
		}()
	}

	orderServer := server.NewOrderServer(repo, publisher, limiter)

	lis, err := net.Listen("tcp", cfg.GRPCServerPort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", cfg.GRPCServerPort, err)
	}

	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, orderServer)

	go func() {
		log.Printf("gRPC API Server running and accepting requests on %s...", cfg.GRPCServerPort)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("gRPC server crashed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	log.Printf("Received shutdown signal (%v). Initiating graceful shutdown...", sig)

	grpcServer.GracefulStop()
	log.Println("gRPC API Server stopped cleanly.")
}
