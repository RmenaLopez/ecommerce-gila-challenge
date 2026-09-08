package service

import (
	"context"
	"strings"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

type ProductService struct {
	repo repository.ProductRepository
}

func NewProductService(repo repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(ctx context.Context, p *domain.Product) error {
	p.SKU = strings.TrimSpace(p.SKU)
	if p.SKU == "" {
		return domain.ErrInvalidInput
	}

	return s.repo.Create(ctx, p)
}

func (s *ProductService) Get(ctx context.Context, id string) (*domain.Product, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProductService) Update(ctx context.Context, p *domain.Product) error {
	return s.repo.Update(ctx, p)
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *ProductService) SearchByName(ctx context.Context, filter repository.ProductFilter) ([]domain.Product, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return s.repo.SearchByName(ctx, filter)
}
