package postgres_test

import (
	"context"
	"errors"
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

func TestProductRepository_Create_DuplicateSKU(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	sku := fmt.Sprintf("TEST-CREATE-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE sku = $1", sku)
	})

	first := domain.Product{SKU: sku, Name: "Original", Category: "Test", PriceCents: 1000, Stock: 5, WeightKg: 1}
	if err := repo.Create(ctx, &first); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	second := domain.Product{SKU: sku, Name: "Duplicate", Category: "Test", PriceCents: 2000, Stock: 9, WeightKg: 1}
	err := repo.Create(ctx, &second)

	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("err = %v, want domain.ErrConflict", err)
	}
}

func TestProductRepository_Update(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	sku := fmt.Sprintf("TEST-UPDATE-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE sku = $1", sku)
	})

	original := domain.Product{SKU: sku, Name: "Original", Category: "Test", PriceCents: 1000, Stock: 5, WeightKg: 1}
	if err := repo.Create(ctx, &original); err != nil {
		t.Fatalf("Create: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // guarantee updated_at can't tie with created_at

	update := domain.Product{
		ID: original.ID, SKU: "SOMETHING-ELSE", Name: "Updated", Category: "Updated Category",
		PriceCents: 2000, Stock: 9, WeightKg: 2,
	}
	if err := repo.Update(ctx, &update); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if update.Name != "Updated" || update.Category != "Updated Category" || update.PriceCents != 2000 || update.Stock != 9 {
		t.Errorf("mutable fields not applied: %+v", update)
	}
	if update.SKU != sku {
		t.Errorf("SKU = %q, want unchanged %q (SKU is immutable via Update)", update.SKU, sku)
	}
	if !update.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt changed: got %v, want unchanged %v", update.CreatedAt, original.CreatedAt)
	}
	if !update.UpdatedAt.After(update.CreatedAt) {
		t.Errorf("UpdatedAt (%v) should be after CreatedAt (%v)", update.UpdatedAt, update.CreatedAt)
	}

	var dbSKU string
	if err := pool.QueryRow(ctx, "SELECT sku FROM products WHERE id = $1", original.ID).Scan(&dbSKU); err != nil {
		t.Fatalf("fetching row: %v", err)
	}
	if dbSKU != sku {
		t.Errorf("sku in database = %q, want unchanged %q", dbSKU, sku)
	}
}

func TestProductRepository_Update_NotFound(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	err := repo.Update(ctx, &domain.Product{ID: "00000000-0000-0000-0000-000000000000", Name: "Nope"})

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound", err)
	}
}

func TestProductRepository_Delete(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	sku := fmt.Sprintf("TEST-DELETE-%d", time.Now().UnixNano())
	product := domain.Product{SKU: sku, Name: "To Be Deleted", Category: "Test", PriceCents: 1000, Stock: 5, WeightKg: 1}
	if err := repo.Create(ctx, &product); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, product.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM products WHERE id = $1", product.ID).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 0 {
		t.Errorf("row still exists after Delete")
	}
}

func TestProductRepository_Delete_NotFound(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound", err)
	}
}

func TestProductRepository_GetBySKU(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	sku := fmt.Sprintf("TEST-GETBYSKU-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM products WHERE sku = $1", sku)
	})

	created := domain.Product{SKU: sku, Name: "Findable", Category: "Test", PriceCents: 1000, Stock: 5, WeightKg: 1}
	if err := repo.Create(ctx, &created); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetBySKU(ctx, sku)
	if err != nil {
		t.Fatalf("GetBySKU: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("ID = %q, want %q", found.ID, created.ID)
	}
	if found.Name != "Findable" {
		t.Errorf("Name = %q, want %q", found.Name, "Findable")
	}
}

func TestProductRepository_GetBySKU_NotFound(t *testing.T) {
	pool := mustTestPool(t)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	_, err := repo.GetBySKU(ctx, "NO-SUCH-SKU")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want domain.ErrNotFound", err)
	}
}
