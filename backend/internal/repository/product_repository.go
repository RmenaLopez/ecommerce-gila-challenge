package repository

import (
	"context"

	"ecommerce-backend/internal/domain"
)

type ProductFilter struct {
	SearchQuery string
	Category    string
	Limit       int
	Offset      int
}

type ProductRepository interface {
	Create(ctx context.Context, p *domain.Product) error
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	GetBySKU(ctx context.Context, sku string) (*domain.Product, error)
	Update(ctx context.Context, p *domain.Product) error
	Delete(ctx context.Context, id string) error
	SearchByName(ctx context.Context, filter ProductFilter) ([]domain.Product, error)
	BulkUpsert(ctx context.Context, products []domain.Product) error
}
