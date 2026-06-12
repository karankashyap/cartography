package woocommerce

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

var skippedStatuses = map[string]bool{
	"cancelled": true,
	"refunded":  true,
	"failed":    true,
}

// ParseOrdersCSV parses a WooCommerce orders CSV export.
// WooCommerce repeats the order row once per line item — rows are grouped by Order Number.
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

	type orderAccum struct {
		order    ingestcore.NormalizedOrder
		customer ingestcore.NormalizedCustomer
		rowCount int
	}

	// Preserve insertion order with a slice of keys.
	orderKeys := []string{}
	orders := map[string]*orderAccum{}

	result := &ingestcore.ParseResult{}

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

		get := func(col string) string {
			i, ok := idx[col]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		orderNum := get("Order Number")
		if orderNum == "" {
			result.RowsSkipped++
			result.Warnings = append(result.Warnings, "missing Order Number — row skipped")
			continue
		}

		status := strings.ToLower(get("Order Status"))
		if skippedStatuses[status] {
			// Count once per unique order, not per item row.
			if _, seen := orders[orderNum]; !seen {
				result.RowsSkipped++
			}
			// Mark as skipped so we don't add it later.
			orders[orderNum] = nil
			continue
		}

		// If previously marked nil (cancelled), stay skipped.
		if acc, exists := orders[orderNum]; exists && acc == nil {
			continue
		}

		item, itemWarn := parseItem(get("Item SKU"), get("Item Name"), get("Item Quantity"), get("Item Cost"))
		if itemWarn != "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("order %s: %s", orderNum, itemWarn))
		}

		if _, exists := orders[orderNum]; !exists {
			orderedAt, err := parseWooTime(get("Order Date"))
			if err != nil {
				result.RowsSkipped++
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("order %s: unrecognized date — row skipped", orderNum))
				orders[orderNum] = nil
				continue
			}

			email := get("Customer Email")
			emailHash := hashString(strings.ToLower(strings.TrimSpace(email)))
			customerExtID := email
			if email == "" {
				emailHash = hashString(orderNum)
				customerExtID = "anon:" + hashString(orderNum)[:16]
			}

			acc := &orderAccum{
				order: ingestcore.NormalizedOrder{
					ExternalID:    orderNum,
					CustomerExtID: customerExtID,
					OrderedAt:     orderedAt,
					TotalCents:    parseMoneyCents(get("Order Total")),
					SubtotalCents: parseMoneyCents(get("Order Total")),
					Currency:      "USD",
					Country:       get("Billing Country"),
				},
				customer: ingestcore.NormalizedCustomer{
					ExternalID:      customerExtID,
					EmailHash:       emailHash,
					Country:         get("Billing Country"),
					FirstOrderAt:    orderedAt,
					TotalSpentCents: parseMoneyCents(get("Order Total")),
				},
			}
			orders[orderNum] = acc
			orderKeys = append(orderKeys, orderNum)
		}

		if item != nil {
			orders[orderNum].order.Items = append(orders[orderNum].order.Items, *item)
		}
		orders[orderNum].rowCount++
	}

	customerSeen := map[string]bool{}
	for _, key := range orderKeys {
		acc := orders[key]
		if acc == nil {
			continue
		}
		result.Store.Orders = append(result.Store.Orders, acc.order)
		result.RowsParsed++

		if !customerSeen[acc.customer.ExternalID] {
			customerSeen[acc.customer.ExternalID] = true
			result.Store.Customers = append(result.Store.Customers, acc.customer)
		}
	}

	return result, nil
}

func parseItem(sku, name, qtyStr, costStr string) (*ingestcore.NormalizedOrderItem, string) {
	if sku == "" && name == "" {
		return nil, "item has no SKU or name"
	}
	qty := parseInt(qtyStr, 1)
	unit := parseMoneyCents(costStr)
	return &ingestcore.NormalizedOrderItem{
		VariantSKU:     sku,
		Quantity:       qty,
		UnitPriceCents: unit,
		LineTotalCents: unit * int64(qty),
	}, ""
}

func buildIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

func parseWooTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"January 2, 2006",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		"01/02/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

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
