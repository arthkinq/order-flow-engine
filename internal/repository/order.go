// Package repository defines storage interfaces and implementations for persisting domain entities.
package repository

import (
	"context"

	"github.com/arthkinq/order-flow-engine/internal/domain"
)

// OrderRepository defines the interface for order persistence and status updates.
type OrderRepository interface {
	// Create saves a new order along with its line items.
	Create(ctx context.Context, order *domain.Order) error

	// GetByID retrieves an oder and its items by order ID.
	GetByID(ctx context.Context, id string) (*domain.Order, error)

	// UpdateStatus updates the order status and optional failure reason.
	UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, failureReason string) error
}
