package config_test

import (
	"testing"

	"github.com/arthkinq/order-flow-engine/internal/config"
	"github.com/ozontech/testo"
	"github.com/stretchr/testify/assert"
)

type ConfigSuite struct {
	testo.Suite[*testo.T]
}

func (ConfigSuite) TestLoad_Defaults(t *testo.T) {
	cfg := config.Load()

	assert.Equal(t, "postgres://postgres:postgres@localhost:5432/order_flow?sslmode=disable", cfg.DBConnString)
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", cfg.RabbitMQURL)
	assert.Equal(t, "orders.direct", cfg.Exchange)
	assert.Equal(t, "orders_queue", cfg.Queue)
	assert.Equal(t, "order.created", cfg.RoutingKey)
	assert.Equal(t, 3, cfg.WorkerCount)
	assert.Equal(t, ":50051", cfg.GRPCServerPort)
	assert.Equal(t, "localhost:50052", cfg.GateKeeperURL)
}

func (ConfigSuite) TestLoad_CustomEnv(t *testo.T) {
	t.Setenv("DB_CONN_STRING", "postgres://custom:pass@db:5432/custom_db")
	t.Setenv("RABBITMQ_URL", "amqp://user:pass@mq:5672/")
	t.Setenv("RABBITMQ_EXCHANGE", "custom.exchange")
	t.Setenv("RABBITMQ_QUEUE", "custom_queue")
	t.Setenv("RABBITMQ_ROUTING_KEY", "custom.key")
	t.Setenv("WORKER_COUNT", "10")
	t.Setenv("GRPC_SERVER_PORT", ":9090")
	t.Setenv("GATEKEEPER_URL", "gatekeeper:9090")

	cfg := config.Load()

	assert.Equal(t, "postgres://custom:pass@db:5432/custom_db", cfg.DBConnString)
	assert.Equal(t, "amqp://user:pass@mq:5672/", cfg.RabbitMQURL)
	assert.Equal(t, "custom.exchange", cfg.Exchange)
	assert.Equal(t, "custom_queue", cfg.Queue)
	assert.Equal(t, "custom.key", cfg.RoutingKey)
	assert.Equal(t, 10, cfg.WorkerCount)
	assert.Equal(t, ":9090", cfg.GRPCServerPort)
	assert.Equal(t, "gatekeeper:9090", cfg.GateKeeperURL)
}

func (ConfigSuite) TestLoad_InvalidWorkerCountFallback(t *testo.T) {
	t.Setenv("WORKER_COUNT", "not-a-number")

	cfg := config.Load()

	assert.Equal(t, 3, cfg.WorkerCount)
}

func TestConfigSuite(t *testing.T) {
	testo.RunSuite(t, new(ConfigSuite))
}
