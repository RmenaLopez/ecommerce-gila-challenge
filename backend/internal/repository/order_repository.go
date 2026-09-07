package repository

import (
	"context"

	"ecommerce-backend/internal/domain"
)

type OrderRepository interface {
	Create(ctx context.Context, o *domain.Order) error
	GetByID(ctx context.Context, id string) (*domain.Order, error)
}
