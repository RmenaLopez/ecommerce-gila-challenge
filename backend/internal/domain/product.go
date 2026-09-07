package domain

import "time"

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
