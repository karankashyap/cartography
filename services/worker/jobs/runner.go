package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner polls for pending import jobs and dispatches them.
type Runner struct {
	db        *pgxpool.Pool
	ollamaURL string
}

func NewRunner(db *pgxpool.Pool, ollamaURL string) *Runner {
	return &Runner{db: db, ollamaURL: ollamaURL}
}

// Run is the main event loop. It listens for Postgres NOTIFY on import_jobs
// and also polls every 5s to catch any missed notifications.
func (r *Runner) Run(ctx context.Context) error {
	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN import_jobs"); err != nil {
		return err
	}
	slog.Info("listening on import_jobs channel")

	// Process any pending jobs at startup
	if err := r.processPending(ctx); err != nil {
		slog.Error("initial pending sweep failed", "err", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.processPending(ctx); err != nil {
				slog.Error("pending sweep failed", "err", err)
			}
		default:
			notif, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.Error("wait for notification", "err", err)
				time.Sleep(time.Second)
				continue
			}
			slog.Info("received import_jobs notification", "payload", notif.Payload)
			if err := r.processJob(ctx, notif.Payload); err != nil {
				slog.Error("process job", "job_id", notif.Payload, "err", err)
			}
		}
	}
}

func (r *Runner) processPending(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `
		SELECT id::TEXT FROM import_jobs
		WHERE state = 'pending'
		ORDER BY created_at
		LIMIT 10
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if err := r.processJob(ctx, id); err != nil {
			slog.Error("process pending job", "job_id", id, "err", err)
		}
	}
	return nil
}
