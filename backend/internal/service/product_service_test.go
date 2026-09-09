package service

import (
	"context"
	"errors"
	"testing"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

// fakeProductRepository is a test double satisfying repository.ProductRepository.
// It only records what SearchByName/Create/BulkUpsert received; every other
// method is an unused stub.
type fakeProductRepository struct {
	receivedFilter repository.ProductFilter

	createCalled       bool
	updateCalled       bool
	bulkUpsertProducts []domain.Product
	receivedSKU        string
}

func (f *fakeProductRepository) Create(ctx context.Context, p *domain.Product) error {
	f.createCalled = true
	return nil
}
func (f *fakeProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	return nil, nil
}
func (f *fakeProductRepository) GetBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	f.receivedSKU = sku
	return nil, nil
}
func (f *fakeProductRepository) Update(ctx context.Context, p *domain.Product) error {
	f.updateCalled = true
	return nil
}
func (f *fakeProductRepository) Delete(ctx context.Context, id string) error         { return nil }
func (f *fakeProductRepository) BulkUpsert(ctx context.Context, products []domain.Product) error {
	f.bulkUpsertProducts = products
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

func TestProductService_Create_RequiresSKU(t *testing.T) {
	cases := []struct {
		name string
		sku  string
	}{
		{"empty sku", ""},
		{"whitespace-only sku", "   "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeProductRepository{}
			svc := NewProductService(repo)

			err := svc.Create(context.Background(), &domain.Product{SKU: c.sku, Name: "Test Product"})

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("err = %v, want domain.ErrInvalidInput", err)
			}
			if repo.createCalled {
				t.Error("expected repository.Create not to be called for an invalid SKU")
			}
		})
	}
}

func TestProductService_Create_TrimsSKU(t *testing.T) {
	repo := &fakeProductRepository{}
	svc := NewProductService(repo)

	p := &domain.Product{SKU: "  RS-001  ", Name: "Test Product"}
	if err := svc.Create(context.Background(), p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.SKU != "RS-001" {
		t.Errorf("SKU = %q, want trimmed %q", p.SKU, "RS-001")
	}
	if !repo.createCalled {
		t.Error("expected repository.Create to be called for a valid SKU")
	}
}

func TestProductService_Create_RejectsInvalidFields(t *testing.T) {
	cases := []struct {
		name    string
		product domain.Product
	}{
		{"empty name", domain.Product{SKU: "SKU-1", Name: ""}},
		{"negative price", domain.Product{SKU: "SKU-1", Name: "Widget", PriceCents: -1}},
		{"negative stock", domain.Product{SKU: "SKU-1", Name: "Widget", Stock: -1}},
		{"negative weight", domain.Product{SKU: "SKU-1", Name: "Widget", WeightKg: -1}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeProductRepository{}
			svc := NewProductService(repo)

			err := svc.Create(context.Background(), &c.product)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("err = %v, want domain.ErrInvalidInput", err)
			}
			if repo.createCalled {
				t.Error("expected repository.Create not to be called for an invalid product")
			}
		})
	}
}

func TestProductService_Update_DoesNotRequireSKU(t *testing.T) {
	repo := &fakeProductRepository{}
	svc := NewProductService(repo)

	// SKU is immutable via Update — a request body that omits it (or sends
	// something different) must still be accepted.
	p := &domain.Product{ID: "some-id", SKU: "", Name: "Widget", PriceCents: 1000, Stock: 5}
	if err := svc.Update(context.Background(), p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.updateCalled {
		t.Error("expected repository.Update to be called")
	}
}

func TestProductService_Update_RejectsInvalidFields(t *testing.T) {
	cases := []struct {
		name    string
		product domain.Product
	}{
		{"empty name", domain.Product{ID: "some-id", Name: ""}},
		{"negative price", domain.Product{ID: "some-id", Name: "Widget", PriceCents: -1}},
		{"negative stock", domain.Product{ID: "some-id", Name: "Widget", Stock: -1}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeProductRepository{}
			svc := NewProductService(repo)

			err := svc.Update(context.Background(), &c.product)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("err = %v, want domain.ErrInvalidInput", err)
			}
			if repo.updateCalled {
				t.Error("expected repository.Update not to be called for an invalid product")
			}
		})
	}
}

func TestProductService_GetBySKU_TrimsInput(t *testing.T) {
	repo := &fakeProductRepository{}
	svc := NewProductService(repo)

	if _, err := svc.GetBySKU(context.Background(), "  RS-001  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.receivedSKU != "RS-001" {
		t.Errorf("repo received SKU = %q, want trimmed %q", repo.receivedSKU, "RS-001")
	}
}
