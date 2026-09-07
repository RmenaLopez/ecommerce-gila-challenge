package service

import (
	"context"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

type OrderService struct {
	products repository.ProductRepository
	orders   repository.OrderRepository
}

func NewOrderService(products repository.ProductRepository, orders repository.OrderRepository) *OrderService {
	return &OrderService{products: products, orders: orders}
}

func (s *OrderService) Purchase(ctx context.Context, items []domain.OrderItem) (*domain.Order, error) {
	return nil, errNotImplemented
}
