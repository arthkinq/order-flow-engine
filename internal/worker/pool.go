package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/queue"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Pool manages a pool of worker goroutines consuming and processing order tasks.
type Pool struct {
	consumer       queue.Consumer
	processor      *OrderProcessor
	workers        int
	maxRetries     int
	baseRetryDelay time.Duration
	wg             sync.WaitGroup
}

// NewPool constructs a new Worker Pool with exponential backoff retry defaults.
func NewPool(consumer queue.Consumer, processor *OrderProcessor, numWorkers int) *Pool {
	if numWorkers <= 0 {
		numWorkers = 3 // default
	}
	return &Pool{
		consumer:       consumer,
		processor:      processor,
		workers:        numWorkers,
		maxRetries:     3,
		baseRetryDelay: 20 * time.Millisecond,
	}
}

// Start launches numWorkers goroutines to process incoming messages concurrently.
func (p *Pool) Start(ctx context.Context) error {
	deliveries, err := p.consumer.Consume()
	if err != nil {
		return fmt.Errorf("worker pool failed to start consumer: %w", err)
	}

	for i := 1; i <= p.workers; i++ {
		p.wg.Add(1)
		go p.workerLoop(ctx, i, deliveries)
	}

	return nil
}

// workerLoop is the body of each worker goroutine.
func (p *Pool) workerLoop(ctx context.Context, workerID int, deliveries <-chan amqp.Delivery) {
	defer p.wg.Done()

	for d := range deliveries {
		var event queue.OrderCreatedEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			slog.ErrorContext(ctx, "failed to unmarshal message, routing to DLQ",
				slog.Int("worker_id", workerID),
				slog.String("error", err.Error()),
			)
			_ = d.Nack(false, false) // requeue = false -> routes message to DLQ
			continue
		}

		slog.InfoContext(ctx, "processing order task",
			slog.Int("worker_id", workerID),
			slog.String("order_id", event.OrderID),
		)

		if err := p.processWithRetry(ctx, workerID, event.OrderID); err != nil {
			slog.ErrorContext(ctx, "order failed after retries, routing to DLQ",
				slog.Int("worker_id", workerID),
				slog.String("order_id", event.OrderID),
				slog.Int("max_retries", p.maxRetries),
				slog.String("error", err.Error()),
			)
			_ = d.Nack(false, false) // Reject and route to DLQ
		} else {
			slog.InfoContext(ctx, "order successfully completed",
				slog.Int("worker_id", workerID),
				slog.String("order_id", event.OrderID),
			)
			_ = d.Ack(false) // Confirm success and remove from queue
		}
	}
}

// processWithRetry executes ProcessOrder with exponential backoff before giving up.
func (p *Pool) processWithRetry(ctx context.Context, workerID int, orderID string) error {
	attempts := p.maxRetries
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = p.processor.ProcessOrder(ctx, orderID)
		if lastErr == nil {
			return nil
		}

		if attempt == attempts || ctx.Err() != nil {
			break
		}

		backoff := p.baseRetryDelay * (1 << (attempt - 1))
		slog.WarnContext(ctx, "transient order processing failure, retrying with backoff",
			slog.Int("worker_id", workerID),
			slog.String("order_id", orderID),
			slog.Int("attempt", attempt),
			slog.Duration("backoff", backoff),
			slog.String("error", lastErr.Error()),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	return lastErr
}

// Stop initiates graceful shutdown, waiting for all in-flight jobs to complete.
func (p *Pool) Stop() error {
	err := p.consumer.Close()
	p.wg.Wait()
	return err
}
