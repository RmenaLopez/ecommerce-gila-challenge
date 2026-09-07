package postgres

import (
	"context"

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

func (r *OrderRepository) Create(ctx context.Context, o *domain.Order) error {
	return errNotImplemented
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	return nil, errNotImplemented
}
