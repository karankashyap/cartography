package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricsFilter struct {
	StoreID uuid.UUID
	From    time.Time
	To      time.Time
}

type Metrics struct {
	RevenueCents       int64
	Orders             int
	AOVCents           int64
	UnitsSold          int
	NewCustomers       int
	ReturningCustomers int
	ReturningRate      float64
	TopProducts        []ProductStat
	BottomProducts     []ProductStat
	DeadStock          []VariantStat
	Trend              []TimePoint
	CohortRetention    []CohortRow
	InventoryVelocity  []VelocityStat
}

type ProductStat struct {
	ProductID    string
	Title        string
	RevenueCents int64
	UnitsSold    int
}

type VariantStat struct {
	VariantID         string
	SKU               string
	Title             string
	InventoryQty      int
	DaysSinceLastSale int
}

type TimePoint struct {
	Date         string
	RevenueCents int64
	Orders       int
}

// Compute runs all sub-queries and assembles Metrics. V7: all computation is SQL/Go, never LLM.
func Compute(ctx context.Context, db *pgxpool.Pool, f MetricsFilter) (*Metrics, error) {
	if f.From.IsZero() {
		f.From = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if f.To.IsZero() {
		f.To = time.Now().UTC()
	}

	m := &Metrics{}

	if err := computeSummary(ctx, db, f, m); err != nil {
		return nil, fmt.Errorf("analytics summary: %w", err)
	}
	if err := computeTopProducts(ctx, db, f, m); err != nil {
		return nil, fmt.Errorf("analytics top products: %w", err)
	}
	if err := computeDeadStock(ctx, db, f, m); err != nil {
		return nil, fmt.Errorf("analytics dead stock: %w", err)
	}
	if err := computeTrend(ctx, db, f, m); err != nil {
		return nil, fmt.Errorf("analytics trend: %w", err)
	}

	cohort, err := ComputeCohortRetention(ctx, db, f.StoreID)
	if err != nil {
		return nil, fmt.Errorf("analytics cohort: %w", err)
	}
	m.CohortRetention = cohort

	velocity, err := ComputeInventoryVelocity(ctx, db, f.StoreID)
	if err != nil {
		return nil, fmt.Errorf("analytics velocity: %w", err)
	}
	m.InventoryVelocity = velocity

	if m.Orders > 0 {
		m.AOVCents = m.RevenueCents / int64(m.Orders)
	}
	if total := m.NewCustomers + m.ReturningCustomers; total > 0 {
		m.ReturningRate = float64(m.ReturningCustomers) / float64(total)
	}

	return m, nil
}

func computeSummary(ctx context.Context, db *pgxpool.Pool, f MetricsFilter, m *Metrics) error {
	row := db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(o.total_cents), 0)                             AS revenue,
			COUNT(*)                                                     AS orders,
			COUNT(*) FILTER (WHERE NOT o.is_returning)                  AS new_customers,
			COUNT(*) FILTER (WHERE o.is_returning)                      AS returning_customers,
			COALESCE((SELECT SUM(oi.quantity)
			          FROM order_items oi
			          JOIN orders o2 ON o2.id = oi.order_id
			          WHERE o2.store_id = $1
			            AND o2.ordered_at BETWEEN $2 AND $3), 0)        AS units_sold
		FROM orders o
		WHERE o.store_id = $1
		  AND o.ordered_at BETWEEN $2 AND $3
	`, f.StoreID, f.From, f.To)

	return row.Scan(&m.RevenueCents, &m.Orders, &m.NewCustomers, &m.ReturningCustomers, &m.UnitsSold)
}

func computeTopProducts(ctx context.Context, db *pgxpool.Pool, f MetricsFilter, m *Metrics) error {
	const q = `
		SELECT p.id::TEXT,
		       p.title,
		       COALESCE(SUM(oi.line_total_cents), 0) AS revenue,
		       COALESCE(SUM(oi.quantity), 0)         AS units
		FROM products p
		JOIN variants v ON v.product_id = p.id
		JOIN order_items oi ON oi.variant_id = v.id
		JOIN orders o ON o.id = oi.order_id
		  AND o.store_id = $1
		  AND o.ordered_at BETWEEN $2 AND $3
		WHERE p.store_id = $1
		GROUP BY p.id, p.title
		HAVING COALESCE(SUM(oi.line_total_cents), 0) > 0
		ORDER BY revenue %s
		LIMIT 10
	`

	scan := func(order string) ([]ProductStat, error) {
		rows, err := db.Query(ctx, fmt.Sprintf(q, order), f.StoreID, f.From, f.To)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var stats []ProductStat
		for rows.Next() {
			var s ProductStat
			if err := rows.Scan(&s.ProductID, &s.Title, &s.RevenueCents, &s.UnitsSold); err != nil {
				return nil, err
			}
			stats = append(stats, s)
		}
		return stats, rows.Err()
	}

	top, err := scan("DESC")
	if err != nil {
		return err
	}
	m.TopProducts = top

	bottom, err := scan("ASC")
	if err != nil {
		return err
	}
	m.BottomProducts = bottom
	return nil
}

const deadStockDays = 90

func computeDeadStock(ctx context.Context, db *pgxpool.Pool, f MetricsFilter, m *Metrics) error {
	rows, err := db.Query(ctx, `
		SELECT v.id::TEXT,
		       COALESCE(v.sku, ''),
		       p.title,
		       v.inventory_qty,
		       COALESCE(
		           EXTRACT(DAY FROM now() - MAX(o.ordered_at))::INT,
		           $3
		       ) AS days_since_last_sale
		FROM variants v
		JOIN products p ON p.id = v.product_id
		LEFT JOIN order_items oi ON oi.variant_id = v.id
		LEFT JOIN orders o ON o.id = oi.order_id AND o.store_id = $1
		WHERE p.store_id = $1
		  AND v.inventory_qty > 0
		GROUP BY v.id, v.sku, p.title, v.inventory_qty
		HAVING MAX(o.ordered_at) IS NULL
		    OR MAX(o.ordered_at) < now() - ($2 * INTERVAL '1 day')
		ORDER BY days_since_last_sale DESC, v.inventory_qty DESC
		LIMIT 20
	`, f.StoreID, deadStockDays, deadStockDays)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var s VariantStat
		if err := rows.Scan(&s.VariantID, &s.SKU, &s.Title, &s.InventoryQty, &s.DaysSinceLastSale); err != nil {
			return err
		}
		m.DeadStock = append(m.DeadStock, s)
	}
	return rows.Err()
}

func computeTrend(ctx context.Context, db *pgxpool.Pool, f MetricsFilter, m *Metrics) error {
	rows, err := db.Query(ctx, `
		SELECT DATE_TRUNC('day', ordered_at)::DATE::TEXT AS date,
		       SUM(total_cents)                          AS revenue,
		       COUNT(*)                                  AS orders
		FROM orders
		WHERE store_id = $1
		  AND ordered_at BETWEEN $2 AND $3
		GROUP BY 1
		ORDER BY 1
	`, f.StoreID, f.From, f.To)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p TimePoint
		if err := rows.Scan(&p.Date, &p.RevenueCents, &p.Orders); err != nil {
			return err
		}
		m.Trend = append(m.Trend, p)
	}
	return rows.Err()
}
