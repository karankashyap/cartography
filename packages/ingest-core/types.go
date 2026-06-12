package ingestcore

import "time"

// NormalizedStore is the top-level container returned by any importer.
type NormalizedStore struct {
	Name      string
	Platform  string
	Products  []NormalizedProduct
	Orders    []NormalizedOrder
	Customers []NormalizedCustomer
}

type NormalizedProduct struct {
	ExternalID  string
	Title       string
	Description string
	ProductType string
	Vendor      string
	Tags        []string
	Status      string
	CreatedAt   time.Time
	Variants    []NormalizedVariant
}

type NormalizedVariant struct {
	SKU          string
	OptionColor  string
	OptionSize   string
	OptionOther  string
	PriceCents   int64
	CostCents    *int64
	InventoryQty int
}

type NormalizedCustomer struct {
	ExternalID      string
	EmailHash       string
	Country         string
	FirstOrderAt    time.Time
	OrdersCount     int
	TotalSpentCents int64
}

type NormalizedOrder struct {
	ExternalID    string
	CustomerExtID string
	OrderedAt     time.Time
	SubtotalCents int64
	TotalCents    int64
	Currency      string
	Country       string
	IsReturning   bool
	Items         []NormalizedOrderItem
}

type NormalizedOrderItem struct {
	VariantSKU     string
	Quantity       int
	UnitPriceCents int64
	LineTotalCents int64
}

// ParseResult is returned by every platform parser.
type ParseResult struct {
	Store       NormalizedStore
	RowsParsed  int
	RowsSkipped int
	Warnings    []string
}
