// Package main is the entry point for the Order Flow Engine gRPC API server process.
package main

import (
	"database/sql"
	"errors"
	"log/slog"
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
	"github.com/arthkinq/order-flow-engine/migrations"
	orderv1 "github.com/arthkinq/order-flow-engine/proto/order/v1"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()
	slog.Info("starting Order Flow Engine API Server", slog.String("port", cfg.GRPCServerPort))

	metrics.StartServer(":2112")

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
		slog.Error("database ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("connected to PostgreSQL successfully")

	if err := migrate.Up(db); err != nil {
		slog.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
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
		slog.Error("failed to initialize RabbitMQ publisher", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			slog.Error("error closing publisher", slog.String("error", err.Error()))
		}
	}()

	limiter, err := ratelimit.NewGateKeeperClient(cfg.GateKeeperURL)
	if err != nil {
		slog.Warn("failed to create GateKeeper client", slog.String("error", err.Error()))
	} else {
		defer func() {
			if err := limiter.Close(); err != nil {
				slog.Error("error closing limiter", slog.String("error", err.Error()))
			}
		}()
	}

	orderServer := server.NewOrderServer(repo, publisher, limiter)

	lis, err := net.Listen("tcp", cfg.GRPCServerPort)
	if err != nil {
		slog.Error("failed to listen on port", slog.String("port", cfg.GRPCServerPort), slog.String("error", err.Error()))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, orderServer)

	go func() {
		slog.Info("gRPC API Server running and accepting requests", slog.String("port", cfg.GRPCServerPort))
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			slog.Error("gRPC server crashed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	slog.Info("received shutdown signal, initiating graceful shutdown", slog.String("signal", sig.String()))

	grpcServer.GracefulStop()
	slog.Info("gRPC API Server stopped cleanly")
}
