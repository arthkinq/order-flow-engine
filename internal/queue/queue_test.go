package queue_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/queue"
	"github.com/ozontech/testo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type QueueSuite struct {
	testo.Suite[*testo.T]
}

func (QueueSuite) TestOrderCreatedEvent_Serialization(t *testo.T) {
	now := time.Now().Truncate(time.Second)
	event := queue.OrderCreatedEvent{
		OrderID:   "order-test-999",
		CreatedAt: now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled queue.OrderCreatedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "order-test-999", unmarshaled.OrderID)
	assert.True(t, now.Equal(unmarshaled.CreatedAt.Local()))
}

func (QueueSuite) TestNewRabbitMQPublisher_InvalidConnection(t *testo.T) {
	cfg := queue.SetupConfig{
		URL:        "amqp://guest:guest@127.0.0.1:59999/",
		Exchange:   "orders.direct",
		Queue:      "orders_queue",
		RoutingKey: "order.created",
	}

	pub, err := queue.NewRabbitMQPublisher(cfg)
	assert.Error(t, err)
	assert.Nil(t, pub)
}

func (QueueSuite) TestNewRabbitMQConsumer_InvalidConnection(t *testo.T) {
	cfg := queue.ConsumerConfig{
		URL:           "amqp://guest:guest@127.0.0.1:59999/",
		Queue:         "orders_queue",
		ConsumerTag:   "test-consumer",
		PrefetchCount: 5,
	}

	cons, err := queue.NewRabbitMQConsumer(cfg)
	assert.Error(t, err)
	assert.Nil(t, cons)
}

func TestQueueSuite(t *testing.T) {
	testo.RunSuite(t, new(QueueSuite))
}
