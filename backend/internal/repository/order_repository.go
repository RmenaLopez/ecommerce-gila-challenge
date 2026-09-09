package repository

import (
	"context"

	"ecommerce-backend/internal/domain"
)

type OrderRepository interface {
	// CreatePurchase attempts to fulfill every item atomically: locking and
	// checking stock for each, then either creating the order (if all items
	// are fulfillable) or returning a *domain.PurchaseRejectedError
	// describing every problem found (if any aren't) — never a partial order.
	CreatePurchase(ctx context.Context, items []domain.OrderItem) (*domain.Order, error)
	GetByID(ctx context.Context, id string) (*domain.Order, error)
}
