package shopify_test

import (
	"strings"
	"testing"

	"cartograph/ingest-core/shopify"
)

const sampleOrdersCSV = `Name,Email,Created at,Total,Subtotal,Currency,Billing Country,Customer,Lineitem sku,Lineitem quantity,Lineitem price
#1001,alice@example.com,2024-01-15 10:00:00 +0000,99.00,90.00,USD,US,new,SKU-001,1,99.00
#1002,bob@example.com,2024-01-16 11:00:00 +0000,200.00,185.00,USD,CA,returning,SKU-002,2,100.00
#1003,,2024-01-17 12:00:00 +0000,50.00,45.00,USD,GB,new,SKU-003,1,50.00
`

func TestParseOrdersCSV_BasicParsing(t *testing.T) {
	result, err := shopify.ParseOrdersCSV(strings.NewReader(sampleOrdersCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(result.Store.Orders), 3; got != want {
		t.Errorf("orders: got %d want %d", got, want)
	}
	if got, want := result.RowsParsed, 3; got != want {
		t.Errorf("rows parsed: got %d want %d", got, want)
	}
	if result.RowsSkipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.RowsSkipped)
	}
}

func TestParseOrdersCSV_MoneyCents(t *testing.T) {
	result, err := shopify.ParseOrdersCSV(strings.NewReader(sampleOrdersCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	order := result.Store.Orders[0]
	if got, want := order.TotalCents, int64(9900); got != want {
		t.Errorf("total cents: got %d want %d", got, want)
	}
	if got, want := order.SubtotalCents, int64(9000); got != want {
		t.Errorf("subtotal cents: got %d want %d", got, want)
	}
}

func TestParseOrdersCSV_CustomerDedup(t *testing.T) {
	csv := `Name,Email,Created at,Total,Subtotal,Currency,Billing Country,Customer,Lineitem sku,Lineitem quantity,Lineitem price
#1001,alice@example.com,2024-01-15 10:00:00 +0000,99.00,90.00,USD,US,new,SKU-001,1,99.00
#1002,alice@example.com,2024-01-16 10:00:00 +0000,50.00,45.00,USD,US,returning,SKU-002,1,50.00
`
	result, err := shopify.ParseOrdersCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(result.Store.Customers), 1; got != want {
		t.Errorf("customers: got %d want %d (should dedup same email)", got, want)
	}
}

func TestParseOrdersCSV_EmailHashed(t *testing.T) {
	result, err := shopify.ParseOrdersCSV(strings.NewReader(sampleOrdersCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Store.Customers) == 0 {
		t.Fatal("expected at least 1 customer with email")
	}
	for _, c := range result.Store.Customers {
		if strings.Contains(c.EmailHash, "@") {
			t.Errorf("email not hashed: %q still contains @", c.EmailHash)
		}
		if len(c.EmailHash) != 64 {
			t.Errorf("email hash wrong length: got %d want 64 hex chars, value=%q",
				len(c.EmailHash), c.EmailHash)
		}
	}
}

func TestParseOrdersCSV_ReturningFlag(t *testing.T) {
	result, err := shopify.ParseOrdersCSV(strings.NewReader(sampleOrdersCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Store.Orders[0].IsReturning {
		t.Error("order #1001 should not be returning (Customer=new)")
	}
	if !result.Store.Orders[1].IsReturning {
		t.Error("order #1002 should be returning (Customer=returning)")
	}
}

func TestParseOrdersCSV_MissingNameSkipped(t *testing.T) {
	csv := `Name,Email,Created at,Total,Subtotal,Currency,Billing Country,Customer,Lineitem sku,Lineitem quantity,Lineitem price
,alice@example.com,2024-01-15 10:00:00 +0000,99.00,90.00,USD,US,new,SKU-001,1,99.00
`
	result, err := shopify.ParseOrdersCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(result.Store.Orders); got != 0 {
		t.Errorf("expected 0 orders for missing Name, got %d", got)
	}
	if result.RowsSkipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.RowsSkipped)
	}
}

func TestParseOrdersCSV_CommaInMoney(t *testing.T) {
	csv := `Name,Email,Created at,Total,Subtotal,Currency,Billing Country,Customer,Lineitem sku,Lineitem quantity,Lineitem price
#2001,carol@example.com,2024-03-01 09:00:00 +0000,"1,234.56","1,200.00",USD,US,new,SKU-010,1,"1,234.56"
`
	result, err := shopify.ParseOrdersCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := result.Store.Orders[0].TotalCents, int64(123456); got != want {
		t.Errorf("comma-formatted money: got %d want %d", got, want)
	}
}
