package domain_test

import (
	"errors"
	"testing"

	"ecommerce-backend/internal/domain"
)

func validProduct() domain.Product {
	return domain.Product{
		SKU: "SKU-1", Name: "Widget", Category: "Test",
		PriceCents: 1000, Stock: 5, WeightKg: 1,
	}
}

func TestProduct_Validate(t *testing.T) {
	cases := []struct {
		name    string
		modify  func(p *domain.Product)
		wantErr bool
	}{
		{"valid product", func(p *domain.Product) {}, false},
		{"empty sku", func(p *domain.Product) { p.SKU = "" }, true},
		{"whitespace-only sku", func(p *domain.Product) { p.SKU = "   " }, true},
		{"empty name", func(p *domain.Product) { p.Name = "" }, true},
		{"negative price", func(p *domain.Product) { p.PriceCents = -1 }, true},
		{"negative stock", func(p *domain.Product) { p.Stock = -1 }, true},
		{"negative weight", func(p *domain.Product) { p.WeightKg = -1 }, true},
		{"zero price is valid", func(p *domain.Product) { p.PriceCents = 0 }, false},
		{"zero stock is valid", func(p *domain.Product) { p.Stock = 0 }, false},
		{"empty category is valid", func(p *domain.Product) { p.Category = "" }, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validProduct()
			c.modify(&p)

			err := p.Validate()

			if c.wantErr && !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Validate() = %v, want domain.ErrInvalidInput", err)
			}
			if !c.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestProduct_ValidateForUpdate_DoesNotRequireSKU(t *testing.T) {
	p := validProduct()
	p.SKU = "" // SKU is immutable via Update; an update request may not carry it at all

	if err := p.ValidateForUpdate(); err != nil {
		t.Errorf("ValidateForUpdate() = %v, want nil (sku should not be required)", err)
	}
}

func TestProduct_ValidateForUpdate_StillChecksMutableFields(t *testing.T) {
	p := validProduct()
	p.Name = ""

	if err := p.ValidateForUpdate(); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("ValidateForUpdate() = %v, want domain.ErrInvalidInput", err)
	}
}
