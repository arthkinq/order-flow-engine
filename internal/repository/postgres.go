package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresRepository implements OrderRepository using PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository constructs a new PostgresRepository instance.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new order and its line items inside a single database transaction.
func (r *PostgresRepository) Create(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	const queryOrder = `
		INSERT INTO orders (id, customer_id, total_amount_cents, status, failure_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.ExecContext(ctx, queryOrder,
		order.ID,
		order.CustomerID,
		order.TotalAmountCents,
		string(order.Status),
		order.FailureReason,
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	const queryItem = `
		INSERT INTO order_items (order_id, item_id, quantity, price_cents)
		VALUES ($1, $2, $3, $4)
	`
	for _, item := range order.Items {
		_, err = tx.ExecContext(ctx, queryItem,
			order.ID,
			item.ItemID,
			item.Quantity,
			item.PriceCents,
		)
		if err != nil {
			return fmt.Errorf("failed to insert order item [%s]: %w", item.ItemID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID fetches an order and its items by order ID.
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	const queryOrder = `
		SELECT id, customer_id, total_amount_cents, status, failure_reason, created_at, updated_at
		FROM orders
		WHERE id = $1
	`

	var order domain.Order
	var statusStr string

	err := r.db.QueryRowContext(ctx, queryOrder, id).Scan(
		&order.ID,
		&order.CustomerID,
		&order.TotalAmountCents,
		&statusStr,
		&order.FailureReason,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: order id %s", domain.ErrOrderNotFound, id)
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}
	order.Status = domain.OrderStatus(statusStr)

	const queryItems = `
		SELECT item_id, quantity, price_cents
		FROM order_items
		WHERE order_id = $1
	`
	rows, err := r.db.QueryContext(ctx, queryItems, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ItemID, &item.Quantity, &item.PriceCents); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order items rows: %w", err)
	}

	order.Items = items
	return &order, nil
}

// UpdateStatus updates the order status and failure reason if applicable.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, failureReason string) error {
	const query = `
		UPDATE orders
		SET status = $1, failure_reason = $2, updated_at = $3
		WHERE id = $4
	`

	res, err := r.db.ExecContext(ctx, query, string(status), failureReason, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: order id %s", domain.ErrOrderNotFound, id)
	}

	return nil
}
