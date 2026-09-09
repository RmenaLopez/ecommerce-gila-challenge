package service

import (
	"context"
	"strings"
	"testing"
)

func TestImportService_ImportCSV(t *testing.T) {
	// Mirrors the real edge cases found in the challenge's sample CSV:
	// a $-prefixed price, a non-numeric price, negative stock, an empty
	// name, and a duplicate SKU where the later row should win.
	csvContent := `name,sku,description,category,price,stock,weight_kg
Running Shoes,RS-001,Original,Footwear,89.99,150,0.35
Wireless Mouse,WM-042,Has dollar sign,Electronics,$29.99,75,0.12
Yoga Mat,YM-015,Bad price,Sports,free,200,1.2
Desk Lamp,DL-007,Bad stock,Home,45.50,-5,2.1
,HD-099,No name,Electronics,149.99,30,0.25
Running Shoes,RS-001,Updated,Footwear,94.99,120,0.35
`

	repo := &fakeProductRepository{}
	svc := NewImportService(repo)

	result, err := svc.ImportCSV(context.Background(), strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Imported != 2 { // WM-042 and RS-001 (the updated row)
		t.Errorf("Imported = %d, want 2", result.Imported)
	}
	if result.Skipped != 4 { // free price, negative stock, empty name, superseded RS-001
		t.Errorf("Skipped = %d, want 4", result.Skipped)
	}
	if len(result.Errors) != 4 {
		t.Errorf("len(Errors) = %d, want 4, got %v", len(result.Errors), result.Errors)
	}

	if len(repo.bulkUpsertProducts) != 2 {
		t.Fatalf("BulkUpsert called with %d products, want 2: %+v", len(repo.bulkUpsertProducts), repo.bulkUpsertProducts)
	}

	var foundRS001, foundWM042 bool
	for _, p := range repo.bulkUpsertProducts {
		switch p.SKU {
		case "RS-001":
			foundRS001 = true
			if p.PriceCents != 9499 {
				t.Errorf("RS-001 PriceCents = %d, want 9499 (should be the later, updated row)", p.PriceCents)
			}
			if p.Stock != 120 {
				t.Errorf("RS-001 Stock = %d, want 120 (should be the later, updated row)", p.Stock)
			}
		case "WM-042":
			foundWM042 = true
			if p.PriceCents != 2999 {
				t.Errorf("WM-042 PriceCents = %d, want 2999 (leading $ should be stripped)", p.PriceCents)
			}
		}
	}
	if !foundRS001 {
		t.Error("expected RS-001 in the final batch")
	}
	if !foundWM042 {
		t.Error("expected WM-042 in the final batch")
	}
}

func TestImportService_ImportCSV_MissingColumn(t *testing.T) {
	csvContent := "name,sku,description,category,price,stock\nFoo,F-1,desc,cat,10.00,5\n" // missing weight_kg

	repo := &fakeProductRepository{}
	svc := NewImportService(repo)

	_, err := svc.ImportCSV(context.Background(), strings.NewReader(csvContent))

	if err == nil {
		t.Fatal("expected an error for a CSV missing a required column")
	}
	if repo.bulkUpsertProducts != nil {
		t.Error("BulkUpsert should not have been called")
	}
}
