package domain_test

import (
	"errors"
	"testing"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/ozontech/testo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DomainSuite struct {
	testo.Suite[*testo.T]
}

func (DomainSuite) TestNewOrder_Success(t *testo.T) {
	items := []domain.OrderItem{
		{ItemID: "item-1", Quantity: 2, PriceCents: 500},  // 1000 cents
		{ItemID: "item-2", Quantity: 1, PriceCents: 1500}, // 1500 cents
	}

	order, err := domain.NewOrder("cust-123", items)
	require.NoError(t, err)
	require.NotNil(t, order)

	assert.NotEmpty(t, order.ID)
	assert.Equal(t, "cust-123", order.CustomerID)
	assert.Equal(t, int64(2500), order.TotalAmountCents)
	assert.Equal(t, domain.StatusPending, order.Status)
	assert.Empty(t, order.FailureReason)
	assert.False(t, order.CreatedAt.IsZero())
	assert.False(t, order.UpdatedAt.IsZero())
}

func (DomainSuite) TestNewOrder_ValidationErrors(t *testo.T) {
	tests := []struct {
		name       string
		customerID string
		items      []domain.OrderItem
	}{
		{
			name:       "empty customer ID",
			customerID: "",
			items:      []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 100}},
		},
		{
			name:       "empty items list",
			customerID: "cust-1",
			items:      []domain.OrderItem{},
		},
		{
			name:       "empty item ID",
			customerID: "cust-1",
			items:      []domain.OrderItem{{ItemID: "", Quantity: 1, PriceCents: 100}},
		},
		{
			name:       "zero quantity",
			customerID: "cust-1",
			items:      []domain.OrderItem{{ItemID: "item-1", Quantity: 0, PriceCents: 100}},
		},
		{
			name:       "negative price",
			customerID: "cust-1",
			items:      []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: -50}},
		},
	}

	for _, tt := range tests {
		order, err := domain.NewOrder(tt.customerID, tt.items)
		assert.Nil(t, order, "case: %s", tt.name)
		require.Error(t, err, "case: %s", tt.name)
		assert.True(t, errors.Is(err, domain.ErrInvalidOrder), "case: %s", tt.name)
	}
}

func (DomainSuite) TestOrder_StateTransitions_PendingToCompleted(t *testo.T) {
	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}

	order, err := domain.NewOrder("cust-1", items)
	require.NoError(t, err)

	err = order.TransitionTo(domain.StatusProcessing, "")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, order.Status)

	err = order.TransitionTo(domain.StatusCompleted, "")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, order.Status)
}

func (DomainSuite) TestOrder_StateTransitions_PendingToFailed(t *testo.T) {
	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}

	order, err := domain.NewOrder("cust-1", items)
	require.NoError(t, err)

	err = order.TransitionTo(domain.StatusFailed, "payment gateway timeout")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, order.Status)
	assert.Equal(t, "payment gateway timeout", order.FailureReason)
}

func (DomainSuite) TestOrder_StateTransitions_InvalidTransition(t *testo.T) {
	items := []domain.OrderItem{{ItemID: "item-1", Quantity: 1, PriceCents: 1000}}

	order, err := domain.NewOrder("cust-1", items)
	require.NoError(t, err)

	_ = order.TransitionTo(domain.StatusProcessing, "")
	_ = order.TransitionTo(domain.StatusCompleted, "")

	err = order.TransitionTo(domain.StatusProcessing, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidStatusTransition))
}

func TestDomainSuite(t *testing.T) {
	testo.RunSuite(t, new(DomainSuite))
}
