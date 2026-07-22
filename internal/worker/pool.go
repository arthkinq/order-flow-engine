package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/arthkinq/order-flow-engine/internal/queue"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Pool manages a pool of worker goroutines consuming and processing order tasks.
type Pool struct {
	consumer  queue.Consumer
	processor *OrderProcessor
	workers   int
	wg        sync.WaitGroup
}

// NewPool constructs a new Worker Pool.
func NewPool(consumer queue.Consumer, processor *OrderProcessor, numWorkers int) *Pool {
	if numWorkers <= 0 {
		numWorkers = 3 // default
	}
	return &Pool{
		consumer:  consumer,
		processor: processor,
		workers:   numWorkers,
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
			log.Printf("[Worker-%d] Failed to unmarshal message: %v. Sending to DLQ.", workerID, err)
			_ = d.Nack(false, false) // requeue = false -> routes message to DLQ
			continue
		}

		log.Printf("[Worker-%d] Processing order ID: %s", workerID, event.OrderID)

		if err := p.processor.ProcessOrder(ctx, event.OrderID); err != nil {
			log.Printf("[Worker-%d] Order [%s] failed: %v. Sending to DLQ.", workerID, event.OrderID, err)
			_ = d.Nack(false, false) // Reject and route to DLQ
		} else {
			log.Printf("[Worker-%d] Order [%s] successfully completed.", workerID, event.OrderID)
			_ = d.Ack(false) // Confirm success and remove from queue
		}
	}
}

// Stop initiates graceful shutdown, waiting for all in-flight jobs to complete.
func (p *Pool) Stop() error {
	err := p.consumer.Close()
	p.wg.Wait()
	return err
}
