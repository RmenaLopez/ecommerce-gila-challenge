package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository/postgres"
)

func seedProduct(t *testing.T, repo *postgres.ProductRepository, priceCents int64, stock int) domain.Product {
	t.Helper()
	ctx := context.Background()

	p := domain.Product{
		SKU: fmt.Sprintf("TEST-ORDER-%d", time.Now().UnixNano()), Name: "Test Product",
		Category: "Test", PriceCents: priceCents, Stock: stock, WeightKg: 1,
	}
	if err := repo.Create(ctx, &p); err != nil {
		t.Fatalf("seeding product: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Delete(context.Background(), p.ID)
	})
	return p
}

func TestOrderRepository_CreatePurchase_Success(t *testing.T) {
	pool := mustTestPool(t)
	productRepo := postgres.NewProductRepository(pool)
	orderRepo := postgres.NewOrderRepository(pool)
	ctx := context.Background()

	productA := seedProduct(t, productRepo, 1000, 10)
	productB := seedProduct(t, productRepo, 2000, 5)

	order, err := orderRepo.CreatePurchase(ctx, []domain.OrderItem{
		{ProductID: productA.ID, Quantity: 2},
		{ProductID: productB.ID, Quantity: 1},
	})
	if err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", order.ID) // cascades to order_items
	})

	wantTotal := int64(2*1000 + 1*2000)
	if order.TotalCents != wantTotal {
		t.Errorf("TotalCents = %d, want %d", order.TotalCents, wantTotal)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Errorf("Status = %q, want %q", order.Status, domain.OrderStatusCompleted)
	}
	if len(order.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(order.Items))
	}

	var stockA, stockB int
	if err := pool.QueryRow(ctx, "SELECT stock FROM products WHERE id = $1", productA.ID).Scan(&stockA); err != nil {
		t.Fatalf("fetching productA stock: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT stock FROM products WHERE id = $1", productB.ID).Scan(&stockB); err != nil {
		t.Fatalf("fetching productB stock: %v", err)
	}
	if stockA != 8 {
		t.Errorf("productA stock = %d, want 8 (10 - 2)", stockA)
	}
	if stockB != 4 {
		t.Errorf("productB stock = %d, want 4 (5 - 1)", stockB)
	}
}

func TestOrderRepository_CreatePurchase_RejectsWholePurchaseAndReportsEveryProblem(t *testing.T) {
	pool := mustTestPool(t)
	productRepo := postgres.NewProductRepository(pool)
	orderRepo := postgres.NewOrderRepository(pool)
	ctx := context.Background()

	okProduct := seedProduct(t, productRepo, 1000, 10)
	lowStockProduct := seedProduct(t, productRepo, 500, 1)
	missingProductID := "00000000-0000-0000-0000-000000000000"

	_, err := orderRepo.CreatePurchase(ctx, []domain.OrderItem{
		{ProductID: okProduct.ID, Quantity: 1},
		{ProductID: lowStockProduct.ID, Quantity: 5},
		{ProductID: missingProductID, Quantity: 1},
	})

	var rejected *domain.PurchaseRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("err = %v, want *domain.PurchaseRejectedError", err)
	}
	if len(rejected.Problems) != 2 {
		t.Fatalf("len(Problems) = %d, want 2 (both bad items reported, not just the first): %+v", len(rejected.Problems), rejected.Problems)
	}

	// Nothing should have been persisted or decremented — the whole purchase is rejected.
	// Scoped to this test's own products, not a table-wide count, since other
	// tests' orders may legitimately coexist in the same shared test database.
	var orderItemCount int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM order_items WHERE product_id IN ($1, $2)", okProduct.ID, lowStockProduct.ID,
	).Scan(&orderItemCount); err != nil {
		t.Fatalf("counting order_items: %v", err)
	}
	if orderItemCount != 0 {
		t.Errorf("expected no order_items referencing this test's products, found %d", orderItemCount)
	}

	var okStock int
	if err := pool.QueryRow(ctx, "SELECT stock FROM products WHERE id = $1", okProduct.ID).Scan(&okStock); err != nil {
		t.Fatalf("fetching okProduct stock: %v", err)
	}
	if okStock != 10 {
		t.Errorf("okProduct stock = %d, want unchanged 10 (the whole purchase should have rolled back)", okStock)
	}
}

func TestOrderRepository_CreatePurchase_ConcurrentPurchasesDoNotOversell(t *testing.T) {
	pool := mustTestPool(t)
	productRepo := postgres.NewProductRepository(pool)
	orderRepo := postgres.NewOrderRepository(pool)
	ctx := context.Background()

	// Only one unit in stock; two concurrent purchases both want it.
	product := seedProduct(t, productRepo, 1000, 1)

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := orderRepo.CreatePurchase(ctx, []domain.OrderItem{
				{ProductID: product.ID, Quantity: 1},
			})
			results[i] = err
		}(i)
	}
	wg.Wait()

	successes, rejections := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.As(err, new(*domain.PurchaseRejectedError)):
			rejections++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successes != 1 {
		t.Errorf("successes = %d, want exactly 1 (no overselling)", successes)
	}
	if rejections != 1 {
		t.Errorf("rejections = %d, want exactly 1", rejections)
	}

	var finalStock int
	if err := pool.QueryRow(ctx, "SELECT stock FROM products WHERE id = $1", product.ID).Scan(&finalStock); err != nil {
		t.Fatalf("fetching final stock: %v", err)
	}
	if finalStock != 0 {
		t.Errorf("final stock = %d, want 0", finalStock)
	}
}

func TestOrderRepository_GetByID(t *testing.T) {
	pool := mustTestPool(t)
	productRepo := postgres.NewProductRepository(pool)
	orderRepo := postgres.NewOrderRepository(pool)
	ctx := context.Background()

	product := seedProduct(t, productRepo, 1500, 10)
	created, err := orderRepo.CreatePurchase(ctx, []domain.OrderItem{{ProductID: product.ID, Quantity: 3}})
	if err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", created.ID)
	})

	found, err := orderRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID = %q, want %q", found.ID, created.ID)
	}
	if found.Status != domain.OrderStatusCompleted {
		t.Errorf("Status = %q, want %q", found.Status, domain.OrderStatusCompleted)
	}
	if found.TotalCents != 4500 {
		t.Errorf("TotalCents = %d, want 4500", found.TotalCents)
	}
	if len(found.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(found.Items))
	}
	item := found.Items[0]
	if item.ProductID != product.ID || item.Quantity != 3 || item.UnitPriceCents != 1500 {
		t.Errorf("unexpected item: %+v", item)
	}
	if item.OrderID != created.ID {
		t.Errorf("item.OrderID = %q, want %q", item.OrderID, created.ID)
	}
}

func TestOrderRepository_GetByID_NotFound(t *testing.T) {
	pool := mustTestPool(t)
	orderRepo := postgres.NewOrderRepository(pool)

	_, err := orderRepo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound", err)
	}
}
