package domain

import (
	"fmt"
	"strings"
	"time"
)

// PriceCents stores money as integer cents to avoid floating-point
// rounding errors in price arithmetic.
type Product struct {
	ID          string    `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	PriceCents  int64     `json:"price_cents"`
	Stock       int       `json:"stock"`
	WeightKg    float64   `json:"weight_kg"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate checks whether a product is acceptable for creation — every
// field a new product must have, including SKU.
func (p Product) Validate() error {
	if strings.TrimSpace(p.SKU) == "" {
		return fmt.Errorf("%w: sku is required", ErrInvalidInput)
	}
	return p.ValidateForUpdate()
}

// ValidateForUpdate checks only the fields Update actually changes. SKU is
// deliberately excluded: it's immutable via Update, so whatever a request
// body contains for it is irrelevant and never applied — requiring it here
// would wrongly reject a valid update that simply omits an unused field.
func (p Product) ValidateForUpdate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if p.PriceCents < 0 {
		return fmt.Errorf("%w: price_cents must be non-negative", ErrInvalidInput)
	}
	if p.Stock < 0 {
		return fmt.Errorf("%w: stock must be non-negative", ErrInvalidInput)
	}
	if p.WeightKg < 0 {
		return fmt.Errorf("%w: weight_kg must be non-negative", ErrInvalidInput)
	}
	return nil
}
