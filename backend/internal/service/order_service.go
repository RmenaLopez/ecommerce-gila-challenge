package service

import (
	"context"

	"github.com/google/uuid"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

type OrderService struct {
	orders repository.OrderRepository
}

func NewOrderService(orders repository.OrderRepository) *OrderService {
	return &OrderService{orders: orders}
}

// Purchase validates request structure (non-empty, positive quantities,
// well-formed product IDs) before touching the database — these are
// malformed-request problems, not business-state conflicts, so they fail
// the whole request immediately with domain.ErrInvalidInput rather than
// being collected alongside things like "insufficient stock".
func (s *OrderService) Purchase(ctx context.Context, items []domain.OrderItem) (*domain.Order, error) {
	if len(items) == 0 {
		return nil, domain.ErrInvalidInput
	}
	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, domain.ErrInvalidInput
		}
		if _, err := uuid.Parse(item.ProductID); err != nil {
			return nil, domain.ErrInvalidInput
		}
	}

	return s.orders.CreatePurchase(ctx, items)
}

func (s *OrderService) Get(ctx context.Context, id string) (*domain.Order, error) {
	return s.orders.GetByID(ctx, id)
}
