package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository is an in-memory implementation of OrderRepository for testing.
type mockRepository struct {
	mu     sync.Mutex
	orders map[string]*domain.Order
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		orders: make(map[string]*domain.Order),
	}
}

func (m *mockRepository) Create(ctx context.Context, order *domain.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[order.ID] = order
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return order, nil
}

func (m *mockRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, failureReason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	order.Status = status
	order.FailureReason = failureReason
	return nil
}

func TestProcessOrder_Success(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()

	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}
	order, err := domain.NewOrder("cust-100", items)
	require.NoError(t, err)

	err = repo.Create(ctx, order)
	require.NoError(t, err)

	processor := NewOrderProcessor(repo)
	processor.failureProbability = 0.0 // 0% failure chance for deterministic test
	processor.paymentLatency = 1 * time.Millisecond

	err = processor.ProcessOrder(ctx, order.ID)
	require.NoError(t, err)

	updatedOrder, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, updatedOrder.Status)
	assert.Empty(t, updatedOrder.FailureReason)
}

func TestProcessOrder_PaymentFailure(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()

	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}
	order, err := domain.NewOrder("cust-100", items)
	require.NoError(t, err)

	err = repo.Create(ctx, order)
	require.NoError(t, err)

	processor := NewOrderProcessor(repo)
	processor.failureProbability = 1.0 // 100% failure chance for testing failure path
	processor.paymentLatency = 1 * time.Millisecond

	err = processor.ProcessOrder(ctx, order.ID)
	require.Error(t, err)

	updatedOrder, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, updatedOrder.Status)
	assert.NotEmpty(t, updatedOrder.FailureReason)
}

func TestProcessOrder_NotFound(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()

	processor := NewOrderProcessor(repo)
	err := processor.ProcessOrder(ctx, "non-existent-id")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrOrderNotFound))
}

func TestProcessOrder_IdempotencyAlreadyCompleted(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()

	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}
	order, err := domain.NewOrder("cust-100", items)
	require.NoError(t, err)

	order.Status = domain.StatusCompleted
	err = repo.Create(ctx, order)
	require.NoError(t, err)

	processor := NewOrderProcessor(repo)
	err = processor.ProcessOrder(ctx, order.ID)
	require.NoError(t, err) // Should do nothing and return nil
}
