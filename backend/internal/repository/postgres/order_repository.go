package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

var _ repository.OrderRepository = (*OrderRepository)(nil)

func (r *OrderRepository) CreatePurchase(ctx context.Context, items []domain.OrderItem) (*domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning purchase transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once Commit has succeeded

	var problems []domain.PurchaseProblem
	var resolvedItems []domain.OrderItem
	var totalCents int64

	for _, item := range items {
		var priceCents int64
		var stock int
		err := tx.QueryRow(ctx,
			"SELECT price_cents, stock FROM products WHERE id = $1 FOR UPDATE",
			item.ProductID,
		).Scan(&priceCents, &stock)

		if errors.Is(err, pgx.ErrNoRows) {
			problems = append(problems, domain.PurchaseProblem{
				ProductID: item.ProductID, Quantity: item.Quantity, Reason: "product not found",
			})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("locking product %s: %w", item.ProductID, err)
		}
		if stock < item.Quantity {
			problems = append(problems, domain.PurchaseProblem{
				ProductID: item.ProductID, Quantity: item.Quantity,
				Reason: fmt.Sprintf("insufficient stock: requested %d, available %d", item.Quantity, stock),
			})
			continue
		}

		resolvedItems = append(resolvedItems, domain.OrderItem{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			UnitPriceCents: priceCents,
		})
		totalCents += priceCents * int64(item.Quantity)
	}

	if len(problems) > 0 {
		return nil, &domain.PurchaseRejectedError{Problems: problems}
	}

	for _, item := range resolvedItems {
		if _, err := tx.Exec(ctx,
			"UPDATE products SET stock = stock - $1 WHERE id = $2", item.Quantity, item.ProductID,
		); err != nil {
			return nil, fmt.Errorf("decrementing stock for product %s: %w", item.ProductID, err)
		}
	}

	order := domain.Order{Status: domain.OrderStatusCompleted, TotalCents: totalCents}
	err = tx.QueryRow(ctx,
		"INSERT INTO orders (status, total_cents) VALUES ($1, $2) RETURNING id::text, created_at",
		order.Status, order.TotalCents,
	).Scan(&order.ID, &order.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating order: %w", err)
	}

	for i := range resolvedItems {
		resolvedItems[i].OrderID = order.ID
		err := tx.QueryRow(ctx,
			"INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents) VALUES ($1, $2, $3, $4) RETURNING id::text",
			order.ID, resolvedItems[i].ProductID, resolvedItems[i].Quantity, resolvedItems[i].UnitPriceCents,
		).Scan(&resolvedItems[i].ID)
		if err != nil {
			return nil, fmt.Errorf("creating order item: %w", err)
		}
	}
	order.Items = resolvedItems

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing purchase transaction: %w", err)
	}

	return &order, nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	var order domain.Order
	err := r.pool.QueryRow(ctx,
		"SELECT id::text, status, total_cents, created_at FROM orders WHERE id = $1", id,
	).Scan(&order.ID, &order.Status, &order.TotalCents, &order.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying order: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		"SELECT id::text, order_id::text, product_id::text, quantity, unit_price_cents "+
			"FROM order_items WHERE order_id = $1",
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("querying order items: %w", err)
	}
	defer rows.Close()

	items := []domain.OrderItem{}
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("scanning order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating order items: %w", err)
	}

	order.Items = items
	return &order, nil
}
