// Package worker provides background processing pipeline and worker pool concurrency control.
package worker

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/arthkinq/order-flow-engine/internal/metrics"
	"github.com/arthkinq/order-flow-engine/internal/repository"
)

// OrderProcessor handles order processing pipeline (validation -> state transition -> simulated payment).
type OrderProcessor struct {
	repo               repository.OrderRepository
	failureProbability float64
	paymentLatency     time.Duration
}

// NewOrderProcessor constructs an OrderProcessor instance.
func NewOrderProcessor(repo repository.OrderRepository) *OrderProcessor {
	return &OrderProcessor{
		repo:               repo,
		failureProbability: 0.05,
		paymentLatency:     100 * time.Millisecond,
	}
}

// ProcessOrder executes the business pipeline for a given order ID.
func (p *OrderProcessor) ProcessOrder(ctx context.Context, orderID string) error {
	start := time.Now()
	defer func() {
		metrics.OrderProcessingDuration.Observe(time.Since(start).Seconds())
	}()

	order, err := p.repo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to fetch order [%s]: %w", orderID, err)
	}

	if order.Status == domain.StatusCompleted || order.Status == domain.StatusFailed {
		return nil
	}

	if err := order.TransitionTo(domain.StatusProcessing, ""); err != nil {
		return fmt.Errorf("failed state transition to PROCESSING: %w", err)
	}
	if err := p.repo.UpdateStatus(ctx, order.ID, domain.StatusProcessing, ""); err != nil {
		return fmt.Errorf("failed to update status in db: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.paymentLatency):
	}

	if rand.Float64() < p.failureProbability {
		failureMsg := "payment gateway processing error: insufficient funds or timeout"
		_ = order.TransitionTo(domain.StatusFailed, failureMsg)
		_ = p.repo.UpdateStatus(ctx, order.ID, domain.StatusFailed, failureMsg)

		metrics.OrdersProcessedTotal.WithLabelValues("failed").Inc()
		return fmt.Errorf("payment failed for order [%s]: %s", orderID, failureMsg)
	}

	if err := order.TransitionTo(domain.StatusCompleted, ""); err != nil {
		return fmt.Errorf("failed state transition to COMPLETED: %w", err)
	}
	if err := p.repo.UpdateStatus(ctx, order.ID, domain.StatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update completed status in db: %w", err)
	}

	metrics.OrdersProcessedTotal.WithLabelValues("completed").Inc()
	return nil
}
