package woocommerce_test

import (
	"strings"
	"testing"

	"cartograph/ingest-core/woocommerce"
)

const sampleCSV = `Order Number,Order Date,Order Status,Customer Email,Customer Name,Order Total,Item Name,Item SKU,Item Quantity,Item Cost,Billing Country
101,2024-03-01 10:00:00,completed,alice@example.com,Alice Smith,99.00,Blue Widget,BW-001,1,99.00,US
102,2024-03-02 11:00:00,processing,bob@example.com,Bob Jones,49.99,Red Gadget,RG-002,2,24.99,CA
103,2024-03-03 12:00:00,cancelled,carol@example.com,Carol Lee,19.99,Green Thing,GT-003,1,19.99,GB
104,2024-03-04 13:00:00,refunded,dave@example.com,Dave Kim,30.00,Purple Item,PI-004,1,30.00,AU
105,2024-03-05 14:00:00,failed,eve@example.com,Eve Park,15.00,Orange Obj,OO-005,1,15.00,US
`

func TestParseOrdersCSV_BasicParsing(t *testing.T) {
	result, err := woocommerce.ParseOrdersCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Orders 103/104/105 should be skipped (cancelled/refunded/failed)
	if got, want := len(result.Store.Orders), 2; got != want {
		t.Errorf("orders: got %d want %d", got, want)
	}
	if got, want := result.RowsParsed, 2; got != want {
		t.Errorf("rows parsed: got %d want %d", got, want)
	}
}

func TestParseOrdersCSV_CancelledOrdersSkipped(t *testing.T) {
	result, err := woocommerce.ParseOrdersCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, o := range result.Store.Orders {
		if o.ExternalID == "103" || o.ExternalID == "104" || o.ExternalID == "105" {
			t.Errorf("order %s (cancelled/refunded/failed) should have been skipped", o.ExternalID)
		}
	}
	if result.RowsSkipped != 3 {
		t.Errorf("expected 3 skipped (cancelled+refunded+failed), got %d", result.RowsSkipped)
	}
}

func TestParseOrdersCSV_MultipleLineItems(t *testing.T) {
	csv := `Order Number,Order Date,Order Status,Customer Email,Customer Name,Order Total,Item Name,Item SKU,Item Quantity,Item Cost,Billing Country
201,2024-03-01 10:00:00,completed,alice@example.com,Alice Smith,150.00,Widget A,WA-001,1,80.00,US
201,2024-03-01 10:00:00,completed,alice@example.com,Alice Smith,150.00,Widget B,WB-002,2,35.00,US
202,2024-03-02 11:00:00,completed,bob@example.com,Bob Jones,50.00,Gadget X,GX-003,1,50.00,CA
`
	result, err := woocommerce.ParseOrdersCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(result.Store.Orders), 2; got != want {
		t.Fatalf("orders: got %d want %d", got, want)
	}
	// Order 201 must have 2 line items (WooCommerce repeat rows → grouped)
	order201 := result.Store.Orders[0]
	if order201.ExternalID != "201" {
		t.Fatalf("expected first order to be 201, got %s", order201.ExternalID)
	}
	if got, want := len(order201.Items), 2; got != want {
		t.Errorf("order 201 items: got %d want %d", got, want)
	}
	// Customer dedup: alice appears on both 201 rows — should be 1 customer record
	if got, want := len(result.Store.Customers), 2; got != want {
		t.Errorf("customers: got %d want %d", got, want)
	}
}

func TestParseOrdersCSV_MoneyCents(t *testing.T) {
	result, err := woocommerce.ParseOrdersCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Order 101: total 99.00 → 9900 cents
	if got, want := result.Store.Orders[0].TotalCents, int64(9900); got != want {
		t.Errorf("total cents: got %d want %d", got, want)
	}
}
