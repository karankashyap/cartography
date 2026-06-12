package shopify

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	ingestcore "cartograph/ingest-core"
)

// ParseOrdersCSV parses a Shopify orders CSV export.
// V4: email is SHA-256 hashed at parse time; plain text is never stored.
// V5: callers should upsert by external_id to keep import idempotent.
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

		order, customer, warn, skip := parseOrderRow(row, idx)
		if skip {
			result.RowsSkipped++
			if warn != "" {
				result.Warnings = append(result.Warnings, warn)
			}
			continue
		}

		result.Store.Orders = append(result.Store.Orders, order)
		result.RowsParsed++

		if customer != nil && !customerSeen[customer.ExternalID] {
			customerSeen[customer.ExternalID] = true
			result.Store.Customers = append(result.Store.Customers, *customer)
		}
	}

	return result, nil
}

func parseOrderRow(row []string, idx map[string]int) (
	order ingestcore.NormalizedOrder,
	customer *ingestcore.NormalizedCustomer,
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

	orderID := get("Name")
	if orderID == "" {
		return order, nil, "missing order Name — row skipped", true
	}

	createdAt, err := parseShopifyTime(get("Created at"))
	if err != nil {
		return order, nil,
			fmt.Sprintf("order %s: unrecognized date %q — row skipped", orderID, get("Created at")),
			true
	}

	qty := parseInt(get("Lineitem quantity"), 1)
	unitPrice := parseMoneyCents(get("Lineitem price"))
	total := parseMoneyCents(get("Total"))
	subtotal := parseMoneyCents(get("Subtotal"))

	order = ingestcore.NormalizedOrder{
		ExternalID:    orderID,
		CustomerExtID: get("Email"),
		OrderedAt:     createdAt,
		SubtotalCents: subtotal,
		TotalCents:    total,
		Currency:      firstNonEmpty(get("Currency"), "USD"),
		Country:       get("Billing Country"),
		IsReturning:   strings.EqualFold(get("Customer"), "returning"),
		Items: []ingestcore.NormalizedOrderItem{
			{
				VariantSKU:     get("Lineitem sku"),
				Quantity:       qty,
				UnitPriceCents: unitPrice,
				LineTotalCents: unitPrice * int64(qty),
			},
		},
	}

	email := get("Email")
	if email != "" {
		c := &ingestcore.NormalizedCustomer{
			ExternalID:      email,
			EmailHash:       hashEmail(email),
			Country:         get("Billing Country"),
			TotalSpentCents: total,
			FirstOrderAt:    createdAt,
		}
		customer = c
	}

	return order, customer, "", false
}

func buildIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

// parseShopifyTime handles the common Shopify export timestamp formats.
func parseShopifyTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05 MST",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

// parseMoneyCents converts "1,234.56" or "1234.56" → int64 cents.
// Returns 0 on empty or unparseable input — never panics.
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

func parseInt(s string, defaultVal int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return defaultVal
	}
	return v
}

// hashEmail: V4 — SHA-256 of lowercased, trimmed email, returned as hex.
func hashEmail(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("%x", h)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
