package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

type ProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

var _ repository.ProductRepository = (*ProductRepository)(nil)

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	const query = "INSERT INTO products " +
		"(sku, name, description, category, price_cents, stock, weight_kg) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7)" +
		"RETURNING id::text, created_at, updated_at"

	err := r.pool.QueryRow(ctx, query,
		p.SKU, p.Name, p.Description, p.Category, p.PriceCents, p.Stock, p.WeightKg,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating product: %w", err)
	}

	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	const query = "SELECT id::text, sku, name, description, category, price_cents, stock, weight_kg, created_at, updated_at " +
		"FROM products " +
		"WHERE id = $1"

	var p domain.Product
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category,
		&p.PriceCents, &p.Stock, &p.WeightKg, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying product by id: %w", err)
	}

	return &p, nil
}

func (r *ProductRepository) GetBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	const query = "SELECT id, sku, name, description, category, price_cents, stock, weight_kg, created_at, updated_at" +
		"FROM products " +
		"WHERE sku = $1"
	var p domain.Product
	err := r.pool.QueryRow(ctx, query, sku).Scan(
		&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category,
		&p.PriceCents, &p.Stock, &p.WeightKg, &p.CreatedAt, &p.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying product by id: %w", err)
	}

	return &p, nil
}

func (r *ProductRepository) Update(ctx context.Context, p *domain.Product) error {
	const query = "UPDATE products " +
		"SET name = $2," +
		"description = $3, " +
		"category = $4, " +
		"price_cents = $5, " +
		"stock = $6, " +
		"weight_kg = $7, " +
		"updated_at = $8 " +
		"WHERE id = $1 " +
		"RETURNING updated_at"
	updateDate := time.Now()
	err := r.pool.QueryRow(ctx, query,
		p.ID, p.Name, p.Description, p.Category, p.PriceCents, p.Stock, p.WeightKg, updateDate).Scan(
		&p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("querying product by id: %w", err)
	}

	return nil
}

func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	const query = "DELETE FROM products where id = $1"
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ProductRepository) SearchByName(ctx context.Context, filter repository.ProductFilter) ([]domain.Product, error) {
	query := "SELECT id::text, sku, name, description, category, price_cents, stock, weight_kg, created_at, updated_at FROM products"

	var conditions []string
	var args []any
	argN := 1

	if filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argN))
		args = append(args, filter.Category)
		argN++
	}

	queryArgN := 0
	if filter.SearchQuery != "" {
		conditions = append(conditions, fmt.Sprintf("name %% $%d", argN))
		args = append(args, filter.SearchQuery)
		queryArgN = argN
		argN++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if queryArgN > 0 {
		query += fmt.Sprintf(" ORDER BY similarity(name, $%d) DESC", queryArgN)
	} else {
		query += " ORDER BY name ASC"
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argN, argN+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching products: %w", err)
	}
	defer rows.Close()

	products := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category,
			&p.PriceCents, &p.Stock, &p.WeightKg, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning product row: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating product rows: %w", err)
	}

	return products, nil
}

func (r *ProductRepository) BulkUpsert(ctx context.Context, products []domain.Product) error {
	if len(products) == 0 {
		return nil
	}

	const columnsPerRow = 7
	const updateClause = " ON CONFLICT (sku) DO UPDATE SET " +
		"name = EXCLUDED.name, " +
		"description = EXCLUDED.description, " +
		"category = EXCLUDED.category, " +
		"price_cents = EXCLUDED.price_cents, " +
		"stock = EXCLUDED.stock, " +
		"weight_kg = EXCLUDED.weight_kg, " +
		"updated_at = now()"

	valueGroups := make([]string, 0, len(products))
	args := make([]any, 0, len(products)*columnsPerRow)
	argN := 1

	for _, p := range products {
		valueGroups = append(valueGroups, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			argN, argN+1, argN+2, argN+3, argN+4, argN+5, argN+6,
		))
		args = append(args, p.SKU, p.Name, p.Description, p.Category, p.PriceCents, p.Stock, p.WeightKg)
		argN += columnsPerRow
	}

	query := "INSERT INTO products (sku, name, description, category, price_cents, stock, weight_kg) VALUES " +
		strings.Join(valueGroups, ", ") +
		updateClause

	_, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk upserting products: %w", err)
	}

	return nil
}
