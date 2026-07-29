package server_test

import (
	"context"
	"testing"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/arthkinq/order-flow-engine/internal/server"
	orderv1 "github.com/arthkinq/order-flow-engine/proto/order/v1"
	"github.com/ozontech/testo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockRepo struct {
	orders map[string]*domain.Order
}

func newMockRepo() *mockRepo {
	return &mockRepo{orders: make(map[string]*domain.Order)}
}

func (m *mockRepo) Create(ctx context.Context, order *domain.Order) error {
	m.orders[order.ID] = order
	return nil
}

func (m *mockRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	order, ok := m.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return order, nil
}

func (m *mockRepo) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, failureReason string) error {
	order, ok := m.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	order.Status = status
	order.FailureReason = failureReason
	return nil
}

type mockPublisher struct {
	published []string
}

func (m *mockPublisher) PublishOrderCreated(ctx context.Context, orderID string) error {
	m.published = append(m.published, orderID)
	return nil
}

func (m *mockPublisher) Close() error {
	return nil
}

type mockLimiter struct {
	allowed bool
}

func (m *mockLimiter) Allow(ctx context.Context, clientID, resourceID string) bool {
	return m.allowed
}

func (m *mockLimiter) Close() error {
	return nil
}

type ServerSuite struct {
	testo.Suite[*testo.T]
}

func (ServerSuite) TestCreateOrder_Success(t *testo.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	lim := &mockLimiter{allowed: true}

	srv := server.NewOrderServer(repo, pub, lim)
	ctx := context.Background()

	req := &orderv1.CreateOrderRequest{
		CustomerId: "cust-1",
		Items: []*orderv1.OrderItem{
			{ItemId: "item-1", Quantity: 2, PriceCents: 500},
		},
	}

	resp, err := srv.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.NotEmpty(t, resp.GetOrderId())
	assert.Equal(t, orderv1.OrderStatus_ORDER_STATUS_PENDING, resp.GetStatus())
	assert.Len(t, pub.published, 1)
	assert.Equal(t, resp.GetOrderId(), pub.published[0])
}

func (ServerSuite) TestCreateOrder_RateLimited(t *testo.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	lim := &mockLimiter{allowed: false}

	srv := server.NewOrderServer(repo, pub, lim)
	ctx := context.Background()

	req := &orderv1.CreateOrderRequest{
		CustomerId: "cust-blocked",
		Items: []*orderv1.OrderItem{
			{ItemId: "item-1", Quantity: 1, PriceCents: 100},
		},
	}

	resp, err := srv.CreateOrder(ctx, req)
	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Empty(t, pub.published)
}

func (ServerSuite) TestGetOrder_Success(t *testo.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	lim := &mockLimiter{allowed: true}

	srv := server.NewOrderServer(repo, pub, lim)
	ctx := context.Background()

	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}
	order, _ := domain.NewOrder("cust-1", items)
	_ = repo.Create(ctx, order)

	req := &orderv1.GetOrderRequest{OrderId: order.ID}
	resp, err := srv.GetOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, order.ID, resp.GetOrder().GetId())
	assert.Equal(t, "cust-1", resp.GetOrder().GetCustomerId())
	assert.Equal(t, int64(1000), resp.GetOrder().GetTotalAmountCents())
	assert.Equal(t, orderv1.OrderStatus_ORDER_STATUS_PENDING, resp.GetOrder().GetStatus())
}

func (ServerSuite) TestGetOrder_NotFound(t *testo.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	lim := &mockLimiter{allowed: true}

	srv := server.NewOrderServer(repo, pub, lim)
	ctx := context.Background()

	req := &orderv1.GetOrderRequest{OrderId: "non-existent-id"}
	resp, err := srv.GetOrder(ctx, req)
	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestServerSuite(t *testing.T) {
	testo.RunSuite(t, new(ServerSuite))
}
