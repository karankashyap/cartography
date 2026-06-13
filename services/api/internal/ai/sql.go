package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sqlSchema = `Tables (PostgreSQL, all money in cents):
stores(id UUID PK, name TEXT, platform TEXT, imported_at TIMESTAMPTZ)
products(id UUID PK, store_id UUID FK→stores, external_id TEXT, title TEXT, description TEXT, product_type TEXT, vendor TEXT, tags TEXT[], status TEXT, created_at TIMESTAMPTZ)
variants(id UUID PK, product_id UUID FK→products, sku TEXT, price_cents BIGINT, cost_cents BIGINT, inventory_qty INT)
customers(id UUID PK, store_id UUID FK→stores, external_id TEXT, email_hash TEXT, country TEXT, first_order_at TIMESTAMPTZ, orders_count INT, total_spent_cents BIGINT)
orders(id UUID PK, store_id UUID FK→stores, external_id TEXT, customer_id UUID FK→customers, ordered_at TIMESTAMPTZ, subtotal_cents BIGINT, total_cents BIGINT, currency TEXT, country TEXT, is_returning BOOL)
order_items(id UUID PK, order_id UUID FK→orders, variant_id UUID FK→variants, quantity INT, unit_price_cents BIGINT, line_total_cents BIGINT)`

var forbiddenKeywords = []string{
	"INSERT ", "UPDATE ", "DELETE ", "DROP ", "CREATE ", "TRUNCATE ",
	"ALTER ", "GRANT ", "REVOKE ", "COPY ", "EXECUTE ", "PERFORM ",
	"CALL ", "DO ", "PG_", "INFORMATION_SCHEMA", "$$",
}

// SQLResult holds the outcome of a text-to-SQL ask.
type SQLResult struct {
	SQL         string
	Blocked     bool
	BlockReason string
	Columns     []string
	Rows        [][]*string
	Explanation string
}

// Ask generates SQL for the question, validates it against guardrails,
// executes it against the read-only pool, and returns the result.
// The generated SQL must use $1 as the store_id parameter.
func (c *Client) Ask(ctx context.Context, storeID uuid.UUID, question string) *SQLResult {
	sql, err := c.generateSQL(ctx, question)
	if err != nil {
		return &SQLResult{
			Blocked:     true,
			BlockReason: "AI unavailable — ensure Ollama is running.",
			Columns:     []string{},
			Rows:        [][]*string{},
			Explanation: "Could not generate a query.",
		}
	}

	if err := validateSQL(sql); err != nil {
		return &SQLResult{
			SQL:         sql,
			Blocked:     true,
			BlockReason: err.Error(),
			Columns:     []string{},
			Rows:        [][]*string{},
			Explanation: "Query blocked by safety guardrail.",
		}
	}

	cols, rows, err := executeSQL(ctx, c.readonlyDB, sql, storeID)
	if err != nil {
		return &SQLResult{
			SQL:         sql,
			Blocked:     false,
			Columns:     []string{},
			Rows:        [][]*string{},
			Explanation: fmt.Sprintf("Query execution failed: %v", err),
		}
	}

	return &SQLResult{
		SQL:         sql,
		Blocked:     false,
		Columns:     cols,
		Rows:        rows,
		Explanation: fmt.Sprintf("Found %d row(s).", len(rows)),
	}
}

func (c *Client) generateSQL(ctx context.Context, question string) (string, error) {
	prompt := fmt.Sprintf(`You are a PostgreSQL expert for an e-commerce analytics platform.
Generate a single SQL SELECT query answering the user's question.

SCHEMA:
%s

RULES:
- Output ONLY the SQL — no explanation, no markdown fences, no comments
- ALWAYS filter by store_id using $1 (UUID parameter) e.g. WHERE products.store_id = $1
- Only SELECT statements (no INSERT/UPDATE/DELETE/DROP/CREATE/TRUNCATE/ALTER)
- Do not reference pg_catalog, information_schema, or system tables
- Add LIMIT 100 if result set could be large

QUESTION: %s`, sqlSchema, question)

	raw, err := c.complete(ctx, prompt, 0.1)
	if err != nil {
		return "", err
	}
	return cleanSQL(raw), nil
}

func cleanSQL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var inner []string
		for _, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "```") {
				continue
			}
			inner = append(inner, l)
		}
		raw = strings.TrimSpace(strings.Join(inner, "\n"))
	}
	raw = strings.TrimRight(strings.TrimSpace(raw), ";")
	// DISTINCT + ORDER BY RANDOM() is illegal in Postgres; strip DISTINCT.
	upper := strings.ToUpper(raw)
	if strings.Contains(upper, "SELECT DISTINCT") {
		raw = raw[:strings.Index(upper, "DISTINCT")] + raw[strings.Index(upper, "DISTINCT")+len("DISTINCT "):]
	}
	return raw
}

func validateSQL(sql string) error {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if !strings.HasPrefix(upper, "SELECT") {
		return fmt.Errorf("only SELECT queries are allowed")
	}
	for _, kw := range forbiddenKeywords {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("forbidden keyword in query: %s", strings.TrimSpace(kw))
		}
	}
	if !strings.Contains(sql, "$1") {
		return fmt.Errorf("query must filter by store_id ($1 parameter required)")
	}
	return nil
}

func executeSQL(ctx context.Context, db *pgxpool.Pool, sql string, storeID uuid.UUID) ([]string, [][]*string, error) {
	upper := strings.ToUpper(sql)
	if !strings.Contains(upper, "LIMIT") {
		sql = sql + " LIMIT 100"
	}

	rows, err := db.Query(ctx, sql, storeID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
	}

	var result [][]*string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, nil, err
		}
		row := make([]*string, len(vals))
		for i, v := range vals {
			if v != nil {
				s := fmt.Sprintf("%v", v)
				row[i] = &s
			}
		}
		result = append(result, row)
	}
	return cols, result, rows.Err()
}
