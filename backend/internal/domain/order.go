package domain

import "time"

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
