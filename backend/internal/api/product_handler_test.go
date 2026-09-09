package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ecommerce-backend/internal/api"
	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository/postgres"
	"ecommerce-backend/internal/service"
)

// mustTestServer wires up a real Server backed by real Postgres (DATABASE_URL),
// the same way cmd/api/main.go does, so routing + handler + service +
// repository are all exercised together. Skips if DATABASE_URL isn't set.
func mustTestServer(t *testing.T) (http.Handler, *postgres.ProductRepository) {
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

	productRepo := postgres.NewProductRepository(pool)
	orderRepo := postgres.NewOrderRepository(pool)

	server := api.NewServer(
		service.NewProductService(productRepo),
		service.NewImportService(productRepo),
		service.NewOrderService(orderRepo),
	)

	return server.Routes(), productRepo
}

func TestHandleSearchProducts(t *testing.T) {
	handler, repo := mustTestServer(t)
	ctx := context.Background()

	suffix := time.Now().UnixNano()
	productA := domain.Product{
		SKU: fmt.Sprintf("TEST-SEARCH-A-%d", suffix), Name: "Running Shoes",
		Category: "Footwear", PriceCents: 1000, Stock: 5, WeightKg: 1,
	}
	productB := domain.Product{
		SKU: fmt.Sprintf("TEST-SEARCH-B-%d", suffix), Name: "Wireless Mouse",
		Category: "Electronics", PriceCents: 2000, Stock: 5, WeightKg: 1,
	}
	if err := repo.Create(ctx, &productA); err != nil {
		t.Fatalf("seeding productA: %v", err)
	}
	if err := repo.Create(ctx, &productB); err != nil {
		t.Fatalf("seeding productB: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Delete(context.Background(), productA.ID)
		_ = repo.Delete(context.Background(), productB.ID)
	})

	t.Run("q filters to matching products by name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products?q=running+shoes", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var products []domain.Product
		if err := json.Unmarshal(w.Body.Bytes(), &products); err != nil {
			t.Fatalf("decoding response: %v", err)
		}

		foundA := false
		for _, p := range products {
			if p.SKU == productA.SKU {
				foundA = true
			}
			if p.SKU == productB.SKU {
				t.Errorf("unrelated product %s should not match this search", productB.SKU)
			}
		}
		if !foundA {
			t.Errorf("expected %s in search results, got %+v", productA.SKU, products)
		}
	})

	t.Run("invalid limit returns 400 without querying the database", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products?limit=abc", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400, body=%s", w.Code, w.Body.String())
		}
	})
}
