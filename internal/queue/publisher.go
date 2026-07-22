// Package queue handles RabbitMQ connection, exchange/queue declarations, publishing, and consuming.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// OrderCreatedEvent is the event payload sent over RabbitMQ when an order is created.
type OrderCreatedEvent struct {
	OrderID   string    `json:"order_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Publisher defines the interface for publishing order events to the queue.
type Publisher interface {
	PublishOrderCreated(ctx context.Context, orderID string) error
	Close() error
}

// RabbitMQPublisher implements Publisher using AMQP 0-9-1.
type RabbitMQPublisher struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	exchange   string
	routingKey string
}

// SetupConfig contains parameters for configuring RabbitMQ exchange and queues.
type SetupConfig struct {
	URL        string
	Exchange   string
	Queue      string
	RoutingKey string
}

// NewRabbitMQPublisher establishes connection and declares Exchange, DLQ, Main Queue, and Bindings.
func NewRabbitMQPublisher(cfg SetupConfig) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// 1. Declare Direct Exchange
	err = ch.ExchangeDeclare(
		cfg.Exchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// 2. Declare Dead Letter Queue (DLQ)
	dlqName := cfg.Queue + ".dlq"
	dlqRoutingKey := cfg.RoutingKey + ".dlq"
	_, err = ch.QueueDeclare(
		dlqName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare DLQ: %w", err)
	}

	err = ch.QueueBind(dlqName, dlqRoutingKey, cfg.Exchange, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind DLQ: %w", err)
	}

	// 3. Declare Main Queue with DLQ arguments
	args := amqp.Table{
		"x-dead-letter-exchange":    cfg.Exchange,
		"x-dead-letter-routing-key": dlqRoutingKey,
	}
	_, err = ch.QueueDeclare(
		cfg.Queue,
		true,
		false,
		false,
		false,
		args,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare main queue: %w", err)
	}

	err = ch.QueueBind(cfg.Queue, cfg.RoutingKey, cfg.Exchange, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind main queue: %w", err)
	}

	return &RabbitMQPublisher{
		conn:       conn,
		ch:         ch,
		exchange:   cfg.Exchange,
		routingKey: cfg.RoutingKey,
	}, nil
}

// PublishOrderCreated serializes and publishes an OrderCreatedEvent to RabbitMQ.
func (p *RabbitMQPublisher) PublishOrderCreated(ctx context.Context, orderID string) error {
	event := OrderCreatedEvent{
		OrderID:   orderID,
		CreatedAt: time.Now().UTC(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal order event: %w", err)
	}

	err = p.ch.PublishWithContext(
		ctx,
		p.exchange,
		p.routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message to rabbitmq: %w", err)
	}

	return nil
}

// Close gracefully closes the channel and connection.
func (p *RabbitMQPublisher) Close() error {
	if err := p.ch.Close(); err != nil {
		p.conn.Close()
		return err
	}
	return p.conn.Close()
}
