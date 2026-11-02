package worker

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/arthkinq/order-flow-engine/internal/queue"
	"github.com/ozontech/testo"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAcknowledger struct {
	acks  atomic.Int32
	nacks atomic.Int32
}

func (m *mockAcknowledger) Ack(tag uint64, multiple bool) error {
	m.acks.Add(1)
	return nil
}

func (m *mockAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	m.nacks.Add(1)
	return nil
}

func (m *mockAcknowledger) Reject(tag uint64, requeue bool) error {
	m.nacks.Add(1)
	return nil
}

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
	processor.failureProbability = 0.0
	processor.paymentLatency = 5 * time.Millisecond

	ch := make(chan amqp.Delivery, 10)
	consumer := &mockConsumer{ch: ch}
	acker := &mockAcknowledger{}

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
		Acknowledger: acker,
		Body:         payload,
	}

	time.Sleep(50 * time.Millisecond)

	err = pool.Stop()
	assert.NoError(t, err)

	updatedOrder, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, updatedOrder.Status)
	assert.Equal(t, int32(1), acker.acks.Load())
}

func (WorkerPoolSuite) TestPool_MalformedPayload_RoutedToDLQ(t *testo.T) {
	repo := newMockRepository()
	processor := NewOrderProcessor(repo)

	ch := make(chan amqp.Delivery, 10)
	consumer := &mockConsumer{ch: ch}
	acker := &mockAcknowledger{}

	pool := NewPool(consumer, processor, 1)
	ctx := context.Background()
	require.NoError(t, pool.Start(ctx))

	ch <- amqp.Delivery{
		Acknowledger: acker,
		Body:         []byte("{invalid-json"),
	}
	ch <- amqp.Delivery{
		Acknowledger: acker,
		Body:         []byte(`{"order_id":"missing-order"}`),
	}

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, pool.Stop())

	assert.Equal(t, int32(2), acker.nacks.Load(), "Expected both malformed and failed messages to be NACKed to DLQ")
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
