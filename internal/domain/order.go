// Package domain defines core business entities, status state machine, and validation rules.
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Sentinel domain errors.
var (
	ErrInvalidOrder            = errors.New("invalid order data")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrOrderNotFound           = errors.New("order not found")
)

// OrderStatus represents the current state of an order in the processing pipeline.
type OrderStatus string

const (
	StatusPending    OrderStatus = "PENDING"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusCompleted  OrderStatus = "COMPLETED"
	StatusFailed     OrderStatus = "FAILED"
)

// String returns the string representation of OrderStatus.
func (s OrderStatus) String() string {
	return string(s)
}

// validTransitions defines allowed state machine transitions for orders.
var validTransitions = map[OrderStatus][]OrderStatus{
	StatusPending:    {StatusProcessing, StatusFailed},
	StatusProcessing: {StatusCompleted, StatusFailed},
	StatusCompleted:  {}, // Terminal state
	StatusFailed:     {}, // Terminal state
}

// OrderItem represents a line item in an order.
type OrderItem struct {
	ItemID     string `json:"item_id"`
	Quantity   int32  `json:"quantity"`
	PriceCents int64  `json:"price_cents"`
}

// Order is the primary aggregate root of the Order Flow Engine.
type Order struct {
	ID               string      `json:"id"`
	CustomerID       string      `json:"customer_id"`
	Items            []OrderItem `json:"items"`
	TotalAmountCents int64       `json:"total_amount_cents"`
	Status           OrderStatus `json:"status"`
	FailureReason    string      `json:"failure_reason,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// NewOrder validates input and constructs a new Order with PENDING status.
func NewOrder(customerID string, items []OrderItem) (*Order, error) {
	if customerID == "" {
		return nil, fmt.Errorf("%w: customer_id cannot be empty", ErrInvalidOrder)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: items list cannot be empty", ErrInvalidOrder)
	}

	var totalCents int64
	for i, item := range items {
		if item.ItemID == "" {
			return nil, fmt.Errorf("%w: item [%d] has empty item_id", ErrInvalidOrder, i)
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("%w: item [%s] quantity must be positive", ErrInvalidOrder, item.ItemID)
		}
		if item.PriceCents <= 0 {
			return nil, fmt.Errorf("%w: item [%s] price must be positive", ErrInvalidOrder, item.ItemID)
		}
		totalCents += int64(item.Quantity) * item.PriceCents
	}

	now := time.Now().UTC()
	return &Order{
		ID:               uuid.NewString(),
		CustomerID:       customerID,
		Items:            items,
		TotalAmountCents: totalCents,
		Status:           StatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// CanTransitionTo checks whether transitioning from current status to next status is permitted.
func (o *Order) CanTransitionTo(next OrderStatus) bool {
	allowed, ok := validTransitions[o.Status]
	if !ok {
		return false
	}
	for _, status := range allowed {
		if status == next {
			return true
		}
	}
	return false
}

// TransitionTo updates the order status if the state transition is valid.
func (o *Order) TransitionTo(next OrderStatus, failureReason string) error {
	if !o.CanTransitionTo(next) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatusTransition, o.Status, next)
	}

	o.Status = next
	if next == StatusFailed {
		o.FailureReason = failureReason
	}
	o.UpdatedAt = time.Now().UTC()
	return nil
}
