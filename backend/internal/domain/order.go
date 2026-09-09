package domain

import (
	"fmt"
	"time"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusFailed    OrderStatus = "failed"
)

type OrderItem struct {
	ID             string `json:"id"`
	OrderID        string `json:"order_id"`
	ProductID      string `json:"product_id"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

type Order struct {
	ID         string      `json:"id"`
	Status     OrderStatus `json:"status"`
	TotalCents int64       `json:"total_cents"`
	Items      []OrderItem `json:"items"`
	CreatedAt  time.Time   `json:"created_at"`
}

// PurchaseProblem describes why a single requested item couldn't be
// purchased. A purchase is all-or-nothing: any problem rejects the whole
// request, but every item is checked so the caller sees every problem at
// once rather than discovering them one retry at a time.
type PurchaseProblem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Reason    string `json:"reason"`
}

// PurchaseRejectedError means no order was created because one or more
// requested items couldn't be fulfilled. Wraps ErrConflict so generic
// error-mapping code can still classify it via errors.Is, while callers
// that need the specific reasons can extract them via errors.As.
type PurchaseRejectedError struct {
	Problems []PurchaseProblem
}

func (e *PurchaseRejectedError) Error() string {
	return fmt.Sprintf("purchase rejected: %d problem(s)", len(e.Problems))
}

func (e *PurchaseRejectedError) Unwrap() error {
	return ErrConflict
}
