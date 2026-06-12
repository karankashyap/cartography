package analytics

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CohortRow struct {
	CohortMonth   string
	ActivityMonth string
	Customers     int
	RetentionPct  float64
}

type VelocityStat struct {
	VariantID        string
	SKU              string
	Title            string
	UnitsPer30Days   float64
	UnitsPer90Days   float64
	InventoryQty     int
}

// ComputeCohortRetention returns monthly cohort retention percentages for a store.
func ComputeCohortRetention(ctx context.Context, db *pgxpool.Pool, storeID uuid.UUID) ([]CohortRow, error) {
	rows, err := db.Query(ctx, `
		WITH cohort AS (
		  SELECT customer_id,
		         DATE_TRUNC('month', MIN(ordered_at)) AS cohort_month
		  FROM orders
		  WHERE store_id = $1
		    AND customer_id IS NOT NULL
		  GROUP BY customer_id
		),
		activity AS (
		  SELECT c.cohort_month,
		         DATE_TRUNC('month', o.ordered_at) AS activity_month,
		         COUNT(DISTINCT c.customer_id)      AS customers
		  FROM cohort c
		  JOIN orders o ON o.customer_id = c.customer_id AND o.store_id = $1
		  GROUP BY 1, 2
		)
		SELECT
		  cohort_month::DATE::TEXT,
		  activity_month::DATE::TEXT,
		  customers,
		  ROUND(100.0 * customers / FIRST_VALUE(customers) OVER (
		    PARTITION BY cohort_month ORDER BY activity_month
		  ), 1) AS retention_pct
		FROM activity
		ORDER BY cohort_month, activity_month
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CohortRow
	for rows.Next() {
		var r CohortRow
		if err := rows.Scan(&r.CohortMonth, &r.ActivityMonth, &r.Customers, &r.RetentionPct); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ComputeInventoryVelocity returns units-sold-per-day metrics for each SKU over 30/90 day windows.
func ComputeInventoryVelocity(ctx context.Context, db *pgxpool.Pool, storeID uuid.UUID) ([]VelocityStat, error) {
	rows, err := db.Query(ctx, `
		SELECT
		  v.id::TEXT,
		  COALESCE(v.sku, ''),
		  p.title,
		  ROUND(COALESCE(SUM(oi.quantity) FILTER (
		    WHERE o.ordered_at >= now() - INTERVAL '30 days'
		  ), 0)::NUMERIC / 30, 4)  AS units_per_30,
		  ROUND(COALESCE(SUM(oi.quantity) FILTER (
		    WHERE o.ordered_at >= now() - INTERVAL '90 days'
		  ), 0)::NUMERIC / 90, 4)  AS units_per_90,
		  v.inventory_qty
		FROM variants v
		JOIN products p ON p.id = v.product_id
		LEFT JOIN order_items oi ON oi.variant_id = v.id
		LEFT JOIN orders o ON o.id = oi.order_id AND o.store_id = $1
		WHERE p.store_id = $1
		  AND v.inventory_qty > 0
		GROUP BY v.id, v.sku, p.title, v.inventory_qty
		ORDER BY units_per_30 DESC
		LIMIT 50
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VelocityStat
	for rows.Next() {
		var s VelocityStat
		if err := rows.Scan(&s.VariantID, &s.SKU, &s.Title, &s.UnitsPer30Days, &s.UnitsPer90Days, &s.InventoryQty); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
