// Package config manages application configuration loaded from environment variables with sensible defaults.
package config

import (
	"os"
	"strconv"
)

// Config contains configuration options for the API and Worker services.
type Config struct {
	DBConnString   string
	RabbitMQURL    string
	Exchange       string
	Queue          string
	RoutingKey     string
	WorkerCount    int
	GRPCServerPort string
	GateKeeperURL  string
}

// Load reads configuration from environment variables or returns defaults.
func Load() *Config {
	return &Config{
		DBConnString:   getEnv("DB_CONN_STRING", "postgres://postgres:postgres@localhost:5432/order_flow?sslmode=disable"),
		RabbitMQURL:    getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Exchange:       getEnv("RABBITMQ_EXCHANGE", "orders.direct"),
		Queue:          getEnv("RABBITMQ_QUEUE", "orders_queue"),
		RoutingKey:     getEnv("RABBITMQ_ROUTING_KEY", "order.created"),
		WorkerCount:    getEnvAsInt("WORKER_COUNT", 3),
		GRPCServerPort: getEnv("GRPC_SERVER_PORT", ":50051"),
		GateKeeperURL:  getEnv("GATEKEEPER_URL", "localhost:50052"),
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultValue
}
