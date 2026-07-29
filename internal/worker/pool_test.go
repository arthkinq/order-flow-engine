package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/arthkinq/order-flow-engine/internal/queue"
	"github.com/ozontech/testo"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConsumer struct {
	ch        chan amqp.Delivery
	closed    bool
	failStart bool
}

func (m *mockConsumer) Consume() (<-chan amqp.Delivery, error) {
	if m.failStart {
		return nil, errors.New("consumer connection failed")
	}
	return m.ch, nil
}

func (m *mockConsumer) Close() error {
	if !m.closed {
		m.closed = true
		close(m.ch)
	}
	return nil
}

type WorkerPoolSuite struct {
	testo.Suite[*testo.T]
}

func (WorkerPoolSuite) TestPool_StartAndStop(t *testo.T) {
	repo := newMockRepository()
	processor := NewOrderProcessor(repo)

	ch := make(chan amqp.Delivery, 10)
	consumer := &mockConsumer{ch: ch}

	pool := NewPool(consumer, processor, 2)
	require.NotNil(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	require.NoError(t, err)

	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}
	order, err := domain.NewOrder("cust-pool", items)
	require.NoError(t, err)
	_ = repo.Create(ctx, order)

	payload, _ := json.Marshal(queue.OrderCreatedEvent{
		OrderID:   order.ID,
		CreatedAt: time.Now(),
	})

	ch <- amqp.Delivery{
		Body: payload,
	}

	time.Sleep(200 * time.Millisecond)

	err = pool.Stop()
	assert.NoError(t, err)

	updatedOrder, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.True(t, updatedOrder.Status == domain.StatusCompleted || updatedOrder.Status == domain.StatusFailed)
}

func (WorkerPoolSuite) TestPool_StartFailure(t *testo.T) {
	repo := newMockRepository()
	processor := NewOrderProcessor(repo)
	consumer := &mockConsumer{ch: make(chan amqp.Delivery), failStart: true}

	pool := NewPool(consumer, processor, 0)
	ctx := context.Background()

	err := pool.Start(ctx)
	require.Error(t, err)
}

func TestWorkerPoolSuite(t *testing.T) {
	testo.RunSuite(t, new(WorkerPoolSuite))
}
