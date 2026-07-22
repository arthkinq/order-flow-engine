package queue

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer defines the interface for consuming messages from RabbitMQ.
type Consumer interface {
	Consume() (<-chan amqp.Delivery, error)
	Close() error
}

// RabbitMQConsumer implements Consumer using AMQP 0-9-1.
type RabbitMQConsumer struct {
	conn          *amqp.Connection
	ch            *amqp.Channel
	queue         string
	consumerTag   string
	prefetchCount int
}

// ConsumerConfig holds configuration options for starting a consumer.
type ConsumerConfig struct {
	URL           string
	Queue         string
	ConsumerTag   string
	PrefetchCount int
}

// NewRabbitMQConsumer initializes connection, channel, and configures QoS prefetch count.
func NewRabbitMQConsumer(cfg ConsumerConfig) (*RabbitMQConsumer, error) {
	if cfg.PrefetchCount <= 0 {
		cfg.PrefetchCount = 10
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Set Quality of Service prefetch count for fair dispatch across workers
	err = ch.Qos(
		cfg.PrefetchCount, // prefetch count
		0,                 // prefetch size no limit
		false,             // per consumer channel
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set channel QoS: %w", err)
	}

	return &RabbitMQConsumer{
		conn:          conn,
		ch:            ch,
		queue:         cfg.Queue,
		consumerTag:   cfg.ConsumerTag,
		prefetchCount: cfg.PrefetchCount,
	}, nil
}

// Consume starts consuming messages from the configured queue with manual ACK mode.
func (c *RabbitMQConsumer) Consume() (<-chan amqp.Delivery, error) {
	deliveries, err := c.ch.Consume(
		c.queue,
		c.consumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start consuming queue [%s]: %w", c.queue, err)
	}

	return deliveries, nil
}

// Close gracefully shuts down the consumer channel and connection.
func (c *RabbitMQConsumer) Close() error {
	if err := c.ch.Close(); err != nil {
		c.conn.Close()
		return err
	}
	return c.conn.Close()
}
