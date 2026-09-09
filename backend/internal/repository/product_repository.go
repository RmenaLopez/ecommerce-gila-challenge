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
	// BulkUpsert inserts new products or updates existing ones (matched by
	// SKU). addToStock controls how stock is combined on conflict: true
	// adds the incoming value to the existing stock (e.g. re-importing a
	// restock CSV), false replaces it outright. Every other field is always
	// replaced with the incoming value in both modes.
	BulkUpsert(ctx context.Context, products []domain.Product, addToStock bool) error
}
