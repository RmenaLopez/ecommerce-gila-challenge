package service

import (
	"context"
	"errors"
	"testing"

	"ecommerce-backend/internal/domain"
)

// fakeOrderRepository is a test double satisfying repository.OrderRepository.
// It only records what CreatePurchase received.
type fakeOrderRepository struct {
	createPurchaseCalled bool
	receivedItems        []domain.OrderItem
}

func (f *fakeOrderRepository) CreatePurchase(ctx context.Context, items []domain.OrderItem) (*domain.Order, error) {
	f.createPurchaseCalled = true
	f.receivedItems = items
	return &domain.Order{}, nil
}

func (f *fakeOrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	return nil, nil
}

func TestOrderService_Purchase_RejectsMalformedRequestsWithoutTouchingTheRepository(t *testing.T) {
	validProductID := "b0e1b6e0-6b9d-4b6a-9b2b-1a2b3c4d5e6f"

	cases := []struct {
		name  string
		items []domain.OrderItem
	}{
		{"empty items", []domain.OrderItem{}},
		{"zero quantity", []domain.OrderItem{{ProductID: validProductID, Quantity: 0}}},
		{"negative quantity", []domain.OrderItem{{ProductID: validProductID, Quantity: -1}}},
		{"malformed product id", []domain.OrderItem{{ProductID: "not-a-uuid", Quantity: 1}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeOrderRepository{}
			svc := NewOrderService(repo)

			_, err := svc.Purchase(context.Background(), c.items)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("err = %v, want domain.ErrInvalidInput", err)
			}
			if repo.createPurchaseCalled {
				t.Error("expected CreatePurchase not to be called for a malformed request")
			}
		})
	}
}

func TestOrderService_Purchase_ValidRequestReachesTheRepository(t *testing.T) {
	repo := &fakeOrderRepository{}
	svc := NewOrderService(repo)

	items := []domain.OrderItem{{ProductID: "b0e1b6e0-6b9d-4b6a-9b2b-1a2b3c4d5e6f", Quantity: 2}}
	if _, err := svc.Purchase(context.Background(), items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.createPurchaseCalled {
		t.Fatal("expected CreatePurchase to be called for a valid request")
	}
	if len(repo.receivedItems) != 1 || repo.receivedItems[0].Quantity != 2 {
		t.Errorf("CreatePurchase received unexpected items: %+v", repo.receivedItems)
	}
}
