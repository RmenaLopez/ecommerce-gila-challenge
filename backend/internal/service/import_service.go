package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"ecommerce-backend/internal/domain"
	"ecommerce-backend/internal/repository"
)

type ImportService struct {
	repo repository.ProductRepository
}

func NewImportService(repo repository.ProductRepository) *ImportService {
	return &ImportService{repo: repo}
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

var requiredCSVColumns = []string{"name", "sku", "description", "category", "price", "stock", "weight_kg"}

// pendingProduct tracks a valid, parsed row alongside the CSV line it came
// from, so a later duplicate SKU can report which earlier line it superseded.
type pendingProduct struct {
	product domain.Product
	line    int
}

func (s *ImportService) ImportCSV(ctx context.Context, r io.Reader) (*ImportResult, error) {
	csvReader := csv.NewReader(r)

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("%w: reading CSV header: %v", domain.ErrInvalidInput, err)
	}

	columnIndex := make(map[string]int, len(header))
	for i, name := range header {
		columnIndex[strings.TrimSpace(name)] = i
	}
	for _, col := range requiredCSVColumns {
		if _, ok := columnIndex[col]; !ok {
			return nil, fmt.Errorf("%w: CSV missing required column %q", domain.ErrInvalidInput, col)
		}
	}

	result := &ImportResult{}
	bySKU := make(map[string]pendingProduct)

	line := 1
	for {
		line++
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: reading CSV row %d: %v", domain.ErrInvalidInput, line, err)
		}

		sku := strings.TrimSpace(row[columnIndex["sku"]])
		name := strings.TrimSpace(row[columnIndex["name"]])

		reject := func(reason string) {
			result.Skipped++
			if sku != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("row %d (%s): %s", line, sku, reason))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("row %d: %s", line, reason))
			}
		}

		if sku == "" {
			reject("empty sku")
			continue
		}
		if name == "" {
			reject("empty name")
			continue
		}

		rawPrice := strings.TrimSpace(row[columnIndex["price"]])
		priceFloat, err := strconv.ParseFloat(strings.TrimPrefix(rawPrice, "$"), 64)
		if err != nil {
			reject(fmt.Sprintf("invalid price %q", rawPrice))
			continue
		}
		if priceFloat < 0 {
			reject(fmt.Sprintf("negative price %q", rawPrice))
			continue
		}

		rawStock := strings.TrimSpace(row[columnIndex["stock"]])
		stock, err := strconv.Atoi(rawStock)
		if err != nil {
			reject(fmt.Sprintf("invalid stock %q", rawStock))
			continue
		}
		if stock < 0 {
			reject(fmt.Sprintf("negative stock %q", rawStock))
			continue
		}

		var weight float64
		if rawWeight := strings.TrimSpace(row[columnIndex["weight_kg"]]); rawWeight != "" {
			weight, err = strconv.ParseFloat(rawWeight, 64)
			if err != nil {
				reject(fmt.Sprintf("invalid weight_kg %q", rawWeight))
				continue
			}
			if weight < 0 {
				reject(fmt.Sprintf("negative weight_kg %q", rawWeight))
				continue
			}
		}

		product := domain.Product{
			SKU:         sku,
			Name:        name,
			Description: row[columnIndex["description"]],
			Category:    row[columnIndex["category"]],
			PriceCents:  int64(math.Round(priceFloat * 100)),
			Stock:       stock,
			WeightKg:    weight,
		}

		if existing, ok := bySKU[sku]; ok {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf(
				"row %d (%s): superseded by a later row with the same sku (row %d)", existing.line, sku, line,
			))
		}
		bySKU[sku] = pendingProduct{product: product, line: line}
	}

	batch := make([]domain.Product, 0, len(bySKU))
	for _, p := range bySKU {
		batch = append(batch, p.product)
	}

	if len(batch) > 0 {
		if err := s.repo.BulkUpsert(ctx, batch); err != nil {
			return nil, fmt.Errorf("bulk upserting imported products: %w", err)
		}
	}

	result.Imported = len(batch)
	return result, nil
}
