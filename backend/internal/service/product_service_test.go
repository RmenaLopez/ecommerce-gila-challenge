package service

import (
	"context"
	"testing"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

// fakeProductRepository is a test double satisfying repository.ProductRepository.
// It only records what SearchByName received; every other method is an unused stub.
type fakeProductRepository struct {
	receivedFilter repository.ProductFilter
}

func (f *fakeProductRepository) Create(ctx context.Context, p *domain.Product) error { return nil }
func (f *fakeProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	return nil, nil
}
func (f *fakeProductRepository) GetBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	return nil, nil
}
func (f *fakeProductRepository) Update(ctx context.Context, p *domain.Product) error { return nil }
func (f *fakeProductRepository) Delete(ctx context.Context, id string) error         { return nil }
func (f *fakeProductRepository) BulkUpsert(ctx context.Context, products []domain.Product) error {
	return nil
}

func (f *fakeProductRepository) SearchByName(ctx context.Context, filter repository.ProductFilter) ([]domain.Product, error) {
	f.receivedFilter = filter
	return nil, nil
}

func TestProductService_SearchByName_ClampsLimitAndOffset(t *testing.T) {
	cases := []struct {
		name       string
		inLimit    int
		inOffset   int
		wantLimit  int
		wantOffset int
	}{
		{"zero limit becomes default", 0, 0, 20, 0},
		{"negative limit becomes default", -5, 0, 20, 0},
		{"over-max limit clamps to 100", 500, 0, 100, 0},
		{"in-range limit is unchanged", 50, 0, 50, 0},
		{"negative offset floors to zero", 20, -10, 20, 0},
		{"in-range offset is unchanged", 20, 10, 20, 10},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeProductRepository{}
			svc := NewProductService(repo)

			_, err := svc.SearchByName(context.Background(), repository.ProductFilter{
				Limit:  c.inLimit,
				Offset: c.inOffset,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if repo.receivedFilter.Limit != c.wantLimit {
				t.Errorf("Limit = %d, want %d", repo.receivedFilter.Limit, c.wantLimit)
			}
			if repo.receivedFilter.Offset != c.wantOffset {
				t.Errorf("Offset = %d, want %d", repo.receivedFilter.Offset, c.wantOffset)
			}
		})
	}
}
