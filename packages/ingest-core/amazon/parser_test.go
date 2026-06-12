package amazon_test

import (
	"strings"
	"testing"

	"cartograph/ingest-core/amazon"
)

const sampleCSV = `order-id,order-date,purchase-date,buyer-name,buyer-email,ship-to-country,item-price,quantity,product-name,asin,sku
112-1234567-0000001,2024-03-01T10:00:00Z,2024-03-01T10:00:00Z,Alice Smith,alice@example.com,US,29.99,1,Wireless Mouse,B08N5KWB9H,WM-001
112-1234567-0000002,2024-03-02T11:00:00Z,2024-03-02T11:00:00Z,Bob Jones,bob@example.com,CA,49.99,2,USB Hub,B07XLNM5NS,UH-002
112-1234567-0000003,2024-03-03T12:00:00Z,2024-03-03T12:00:00Z,Carol Lee,,GB,15.00,1,HDMI Cable,B00V5RG4G2,HC-003
`

func TestParseOrdersCSV_BasicParsing(t *testing.T) {
	result, err := amazon.ParseOrdersCSV(strings.NewReader(sampleCSV))
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
	result, err := amazon.ParseOrdersCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 29.99 USD → 2999 cents
	if got, want := result.Store.Orders[0].TotalCents, int64(2999); got != want {
		t.Errorf("decimal→cents: got %d want %d", got, want)
	}
	// 49.99 × 2 qty → 9998 cents line total
	if got, want := result.Store.Orders[1].TotalCents, int64(9998); got != want {
		t.Errorf("qty×price cents: got %d want %d", got, want)
	}
}

func TestParseOrdersCSV_MissingEmail_AnonymousCustomer(t *testing.T) {
	result, err := amazon.ParseOrdersCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Row 3 has no buyer-email — must not crash, must produce anonymous customer
	if result.RowsSkipped != 0 {
		t.Errorf("missing email should not skip row, got %d skipped", result.RowsSkipped)
	}
	// Find the anonymous customer (for order 112-1234567-0000003)
	var found bool
	for _, c := range result.Store.Customers {
		if strings.HasPrefix(c.ExternalID, "anon:") {
			found = true
			if len(c.EmailHash) != 64 {
				t.Errorf("anon customer email hash wrong length: %d", len(c.EmailHash))
			}
			break
		}
	}
	if !found {
		t.Error("expected anonymous customer with 'anon:' prefix ExternalID")
	}
}

func TestParseOrdersCSV_ASINAsProductExternalID(t *testing.T) {
	result, err := amazon.ParseOrdersCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Store.Products) == 0 {
		t.Fatal("expected products parsed from ASIN")
	}
	// Products keyed by ASIN
	asins := map[string]bool{}
	for _, p := range result.Store.Products {
		asins[p.ExternalID] = true
	}
	for _, want := range []string{"B08N5KWB9H", "B07XLNM5NS", "B00V5RG4G2"} {
		if !asins[want] {
			t.Errorf("expected product with ASIN %s", want)
		}
	}
}

func TestParseOrdersCSV_MissingOrderID_Skipped(t *testing.T) {
	csv := `order-id,order-date,buyer-email,ship-to-country,item-price,quantity,product-name,asin,sku
,2024-03-01T10:00:00Z,alice@example.com,US,10.00,1,Widget,B000000001,W-001
`
	result, err := amazon.ParseOrdersCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(result.Store.Orders); got != 0 {
		t.Errorf("expected 0 orders for missing order-id, got %d", got)
	}
	if result.RowsSkipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.RowsSkipped)
	}
}
