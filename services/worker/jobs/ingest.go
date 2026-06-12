package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ingestcore "cartograph/ingest-core"
	"cartograph/ingest-core/shopify"
)

type importJob struct {
	ID       string
	StoreID  *string
	Filename string
	Platform string
}

func (r *Runner) processJob(ctx context.Context, jobID string) error {
	var job importJob
	row := r.db.QueryRow(ctx, `
		SELECT id::TEXT, store_id::TEXT, filename, platform
		FROM import_jobs WHERE id = $1 AND state = 'pending'
	`, jobID)
	if err := row.Scan(&job.ID, &job.StoreID, &job.Filename, &job.Platform); err != nil {
		return nil // job may have already been picked up
	}

	if err := r.markRunning(ctx, job.ID); err != nil {
		return err
	}

	result, err := r.parseFile(ctx, job)
	if err != nil {
		return r.markFailed(ctx, job.ID, err.Error())
	}

	storeID, err := r.upsertStore(ctx, job, result)
	if err != nil {
		return r.markFailed(ctx, job.ID, err.Error())
	}

	if err := r.upsertData(ctx, storeID, result); err != nil {
		return r.markFailed(ctx, job.ID, err.Error())
	}

	return r.markDone(ctx, job.ID, storeID, result)
}

func (r *Runner) parseFile(ctx context.Context, job importJob) (*ingestcore.ParseResult, error) {
	// Files stored under a configurable data dir; default /tmp/cartograph-uploads
	dataDir := os.Getenv("UPLOAD_DIR")
	if dataDir == "" {
		dataDir = "/tmp/cartograph-uploads"
	}

	f, err := os.Open(filepath.Join(dataDir, job.Filename))
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	switch strings.ToUpper(job.Platform) {
	case "SHOPIFY":
		return shopify.ParseOrdersCSV(f)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", job.Platform)
	}
}

func (r *Runner) upsertStore(ctx context.Context, job importJob, result *ingestcore.ParseResult) (string, error) {
	if job.StoreID != nil && *job.StoreID != "" {
		return *job.StoreID, nil
	}

	storeName := strings.TrimSuffix(job.Filename, filepath.Ext(job.Filename))
	var storeID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO stores (name, platform)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		RETURNING id::TEXT
	`, storeName, strings.ToUpper(job.Platform)).Scan(&storeID)
	if err != nil {
		// Store may already exist; fetch it
		err2 := r.db.QueryRow(ctx, `
			SELECT id::TEXT FROM stores WHERE name = $1 AND platform = $2
		`, storeName, strings.ToUpper(job.Platform)).Scan(&storeID)
		if err2 != nil {
			return "", fmt.Errorf("upsert store: %w / %w", err, err2)
		}
	}

	// Bind job to store
	if _, err := r.db.Exec(ctx, `UPDATE import_jobs SET store_id = $1 WHERE id = $2`,
		storeID, job.ID); err != nil {
		slog.Warn("bind job to store failed", "err", err)
	}

	return storeID, nil
}

func (r *Runner) upsertData(ctx context.Context, storeID string, result *ingestcore.ParseResult) error {
	for _, c := range result.Store.Customers {
		if err := upsertCustomer(ctx, r.db, storeID, c); err != nil {
			slog.Warn("upsert customer failed", "err", err)
		}
	}
	for _, o := range result.Store.Orders {
		if err := upsertOrder(ctx, r.db, storeID, o); err != nil {
			slog.Warn("upsert order failed", "external_id", o.ExternalID, "err", err)
		}
	}
	return nil
}

// V5: all upserts use ON CONFLICT (store_id, external_id) — idempotent re-imports.
func upsertCustomer(ctx context.Context, db *pgxpool.Pool, storeID string, c ingestcore.NormalizedCustomer) error {
	_, err := db.Exec(ctx, `
		INSERT INTO customers (store_id, external_id, email_hash, country, first_order_at, orders_count, total_spent_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (store_id, external_id) DO UPDATE SET
			email_hash        = EXCLUDED.email_hash,
			country           = EXCLUDED.country,
			orders_count      = customers.orders_count + 1,
			total_spent_cents = customers.total_spent_cents + EXCLUDED.total_spent_cents
	`, storeID, c.ExternalID, c.EmailHash, c.Country, c.FirstOrderAt, c.OrdersCount, c.TotalSpentCents)
	return err
}

func upsertOrder(ctx context.Context, db *pgxpool.Pool, storeID string, o ingestcore.NormalizedOrder) error {
	var customerID *string
	if o.CustomerExtID != "" {
		var cid string
		err := db.QueryRow(ctx, `
			SELECT id::TEXT FROM customers WHERE store_id = $1 AND external_id = $2
		`, storeID, o.CustomerExtID).Scan(&cid)
		if err == nil {
			customerID = &cid
		}
	}

	var orderID string
	err := db.QueryRow(ctx, `
		INSERT INTO orders (store_id, external_id, customer_id, ordered_at,
		                    subtotal_cents, total_cents, currency, country, is_returning)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (store_id, external_id) DO NOTHING
		RETURNING id::TEXT
	`, storeID, o.ExternalID, customerID, o.OrderedAt,
		o.SubtotalCents, o.TotalCents, o.Currency, o.Country, o.IsReturning).Scan(&orderID)

	if err != nil || orderID == "" {
		return err // already exists, skip items
	}

	for _, item := range o.Items {
		if err := upsertOrderItem(ctx, db, storeID, orderID, item); err != nil {
			slog.Warn("upsert order item failed", "order_id", orderID, "err", err)
		}
	}
	return nil
}

func upsertOrderItem(ctx context.Context, db *pgxpool.Pool, storeID, orderID string, item ingestcore.NormalizedOrderItem) error {
	var variantID *string
	if item.VariantSKU != "" {
		var vid string
		err := db.QueryRow(ctx, `
			SELECT v.id::TEXT
			FROM variants v
			JOIN products p ON p.id = v.product_id
			WHERE p.store_id = $1 AND v.sku = $2
			LIMIT 1
		`, storeID, item.VariantSKU).Scan(&vid)
		if err == nil {
			variantID = &vid
		}
	}

	_, err := db.Exec(ctx, `
		INSERT INTO order_items (order_id, variant_id, quantity, unit_price_cents, line_total_cents)
		VALUES ($1, $2, $3, $4, $5)
	`, orderID, variantID, item.Quantity, item.UnitPriceCents, item.LineTotalCents)
	return err
}

func (r *Runner) markRunning(ctx context.Context, jobID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE import_jobs SET state = 'running', started_at = now() WHERE id = $1
	`, jobID)
	if err != nil {
		return err
	}
	r.notify(ctx, jobID)
	return nil
}

func (r *Runner) markDone(ctx context.Context, jobID, storeID string, result *ingestcore.ParseResult) error {
	warnings, _ := json.Marshal(result.Warnings)
	_, err := r.db.Exec(ctx, `
		UPDATE import_jobs SET
			state       = 'done',
			store_id    = $2,
			rows_parsed = $3,
			rows_skipped = $4,
			warnings    = $5,
			finished_at = now()
		WHERE id = $1
	`, jobID, storeID, result.RowsParsed, result.RowsSkipped, warnings)
	if err != nil {
		return err
	}
	r.notify(ctx, jobID)
	slog.Info("import done", "job_id", jobID, "parsed", result.RowsParsed, "skipped", result.RowsSkipped)
	return nil
}

func (r *Runner) markFailed(ctx context.Context, jobID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE import_jobs SET state = 'failed', error = $2, finished_at = now() WHERE id = $1
	`, jobID, errMsg)
	if err != nil {
		return err
	}
	r.notify(ctx, jobID)
	slog.Error("import failed", "job_id", jobID, "error", errMsg)
	return nil
}

func (r *Runner) notify(ctx context.Context, jobID string) {
	ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := r.db.Exec(ctx2, "SELECT pg_notify('import_progress', $1)", jobID); err != nil {
		slog.Warn("notify failed", "err", err)
	}
}
