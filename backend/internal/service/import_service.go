package service

import (
	"context"
	"io"

	"ecommerce-backend/internal/repository"
)

type ImportService struct {
	repo repository.ProductRepository
}

func NewImportService(repo repository.ProductRepository) *ImportService {
	return &ImportService{repo: repo}
}

type ImportResult struct {
	Imported int
	Skipped  int
	Errors   []string
}

func (s *ImportService) ImportCSV(ctx context.Context, r io.Reader) (*ImportResult, error) {
	return nil, errNotImplemented
}
