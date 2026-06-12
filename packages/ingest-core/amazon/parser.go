package amazon

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	ingestcore "cartograph/ingest-core"
)

// ParseOrdersCSV parses an Amazon Business Report / Order Report CSV export.
// V4: email SHA-256 hashed at parse time.
// V5: callers upsert by external_id for idempotency.
func ParseOrdersCSV(r io.Reader) (*ingestcore.ParseResult, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	idx := buildIndex(header)
	result := &ingestcore.ParseResult{}
	customerSeen := map[string]bool{}
	productSeen := map[string]bool{}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.RowsSkipped++
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("csv read error at row %d: %v", result.RowsParsed+result.RowsSkipped, err))
			continue
		}

		order, customer, product, warn, skip := parseRow(row, idx)
		if skip {
			result.RowsSkipped++
			if warn != "" {
				result.Warnings = append(result.Warnings, warn)
			}
			continue
		}

		result.Store.Orders = append(result.Store.Orders, order)
		result.RowsParsed++

		if !customerSeen[customer.ExternalID] {
			customerSeen[customer.ExternalID] = true
			result.Store.Customers = append(result.Store.Customers, customer)
		}

		if product.ExternalID != "" && !productSeen[product.ExternalID] {
			productSeen[product.ExternalID] = true
			result.Store.Products = append(result.Store.Products, product)
		}
	}

	return result, nil
}

func parseRow(row []string, idx map[string]int) (
	order ingestcore.NormalizedOrder,
	customer ingestcore.NormalizedCustomer,
	product ingestcore.NormalizedProduct,
	warn string,
	skip bool,
) {
	get := func(col string) string {
		i, ok := idx[col]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	orderID := get("order-id")
	if orderID == "" {
		return order, customer, product, "missing order-id — row skipped", true
	}

	orderedAt, err := parseAmazonTime(firstNonEmpty(get("order-date"), get("purchase-date")))
	if err != nil {
		return order, customer, product,
			fmt.Sprintf("order %s: unrecognized date — row skipped", orderID), true
	}

	qty := parseInt(get("quantity"), 1)
	unitPrice := parseMoneyCents(get("item-price"))
	lineTotal := unitPrice * int64(qty)

	order = ingestcore.NormalizedOrder{
		ExternalID:    orderID,
		CustomerExtID: orderID,
		OrderedAt:     orderedAt,
		SubtotalCents: lineTotal,
		TotalCents:    lineTotal,
		Currency:      "USD",
		Country:       get("ship-to-country"),
		Items: []ingestcore.NormalizedOrderItem{
			{
				VariantSKU:     get("sku"),
				Quantity:       qty,
				UnitPriceCents: unitPrice,
				LineTotalCents: lineTotal,
			},
		},
	}

	// V4: hash email; missing email → anonymous via order-id hash
	email := get("buyer-email")
	var emailHash string
	var customerExtID string
	if email != "" {
		emailHash = hashString(strings.ToLower(strings.TrimSpace(email)))
		customerExtID = email
	} else {
		emailHash = hashString(orderID)
		customerExtID = "anon:" + hashString(orderID)[:16]
	}
	order.CustomerExtID = customerExtID

	customer = ingestcore.NormalizedCustomer{
		ExternalID:      customerExtID,
		EmailHash:       emailHash,
		Country:         get("ship-to-country"),
		FirstOrderAt:    orderedAt,
		TotalSpentCents: lineTotal,
	}

	asin := get("asin")
	product = ingestcore.NormalizedProduct{
		ExternalID: asin,
		Title:      get("product-name"),
		Variants: []ingestcore.NormalizedVariant{
			{
				SKU:        get("sku"),
				PriceCents: unitPrice,
			},
		},
	}

	return order, customer, product, "", false
}

func buildIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

func parseAmazonTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"01/02/2006",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

// parseMoneyCents converts decimal USD string "12.34" → 1234 cents.
func parseMoneyCents(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, _, err := new(big.Float).SetPrec(64).Parse(s, 10)
	if err != nil {
		return 0
	}
	cents, _ := new(big.Float).Mul(f, big.NewFloat(100)).Int64()
	return cents
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func parseInt(s string, defaultVal int) int {
	s = strings.TrimSpace(s)
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return defaultVal
	}
	return n
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
