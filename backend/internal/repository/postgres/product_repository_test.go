package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository/postgres"
)

// mustTestPool connects to the database named by DATABASE_URL, or skips the
// test if it isn't set (e.g. running natively without a database available).
// Intended DATABASE_URL: the test-db started by `docker compose --profile test`.
func mustTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; run via `docker compose --profile test run --rm test`")
	}

	pool, err := postgres.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func TestProductRepository_BulkUpsert(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	sku := fmt.Sprintf("TEST-BULK-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE sku = $1", sku)
	})

	// fetchRow queries directly rather than via GetBySKU, which has a known,
	// separate, not-yet-fixed bug — keeps this test scoped to BulkUpsert only.
	fetchRow := func(t *testing.T) (priceCents int64, stock int, createdAt, updatedAt time.Time) {
		t.Helper()
		err := pool.QueryRow(ctx,
			"SELECT price_cents, stock, created_at, updated_at FROM products WHERE sku = $1", sku,
		).Scan(&priceCents, &stock, &createdAt, &updatedAt)
		if err != nil {
			t.Fatalf("fetching row: %v", err)
		}
		return
	}

	t.Run("inserts a new product", func(t *testing.T) {
		err := repo.BulkUpsert(ctx, []domain.Product{
			{SKU: sku, Name: "Test Product", Category: "Test", PriceCents: 1000, Stock: 5, WeightKg: 1},
		})
		if err != nil {
			t.Fatalf("BulkUpsert: %v", err)
		}

		priceCents, stock, createdAt, updatedAt := fetchRow(t)
		if priceCents != 1000 {
			t.Errorf("price_cents = %d, want 1000", priceCents)
		}
		if stock != 5 {
			t.Errorf("stock = %d, want 5", stock)
		}
		if !createdAt.Equal(updatedAt) {
			t.Errorf("expected created_at == updated_at for a fresh insert, got %v != %v", createdAt, updatedAt)
		}
	})

	t.Run("updates the existing product on conflict", func(t *testing.T) {
		_, _, firstCreatedAt, _ := fetchRow(t)

		time.Sleep(10 * time.Millisecond) // guarantee updated_at can't tie with the first insert's timestamp

		err := repo.BulkUpsert(ctx, []domain.Product{
			{SKU: sku, Name: "Test Product", Category: "Test", PriceCents: 2000, Stock: 9, WeightKg: 1},
		})
		if err != nil {
			t.Fatalf("BulkUpsert: %v", err)
		}

		priceCents, stock, createdAt, updatedAt := fetchRow(t)
		if priceCents != 2000 {
			t.Errorf("price_cents = %d, want 2000 (should have been updated)", priceCents)
		}
		if stock != 9 {
			t.Errorf("stock = %d, want 9 (should have been updated)", stock)
		}
		if !createdAt.Equal(firstCreatedAt) {
			t.Errorf("created_at changed on update: got %v, want unchanged %v", createdAt, firstCreatedAt)
		}
		if !updatedAt.After(createdAt) {
			t.Errorf("updated_at (%v) should be after created_at (%v)", updatedAt, createdAt)
		}
	})
}
