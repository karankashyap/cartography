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

const sqlRules = `STRICT RULES — violating any will cause a runtime error:

1. Output ONLY raw SQL — no explanation, no markdown fences (no ` + "```" + `), no comments, no trailing semicolon.
2. Always filter orders/products/customers/variants directly by store_id:
   - orders.store_id = $1
   - products.store_id = $1
   - customers.store_id = $1
   Never rely solely on a JOIN to satisfy the store_id filter.
3. GROUP BY rules (PostgreSQL is strict):
   - Every non-aggregate column in SELECT must appear in GROUP BY.
   - ORDER BY may only reference expressions that appear in GROUP BY or the SELECT alias.
   - Use DATE_TRUNC('month', ordered_at) — NOT EXTRACT — for time grouping; it's both groupable and orderable.
   - Bad:  SELECT EXTRACT(MONTH FROM ordered_at) AS m ... ORDER BY ordered_at
   - Good: SELECT DATE_TRUNC('month', ordered_at) AS month ... GROUP BY 1 ORDER BY 1
4. Window functions + GROUP BY are mutually exclusive at the same query level.
   - To compute running totals, use a subquery or CTE: GROUP BY in inner query, SUM() OVER in outer.
   - Bad:  SELECT ... SUM(x) OVER (...) FROM t GROUP BY y
   - Good: SELECT *, SUM(rev) OVER (ORDER BY month) FROM (SELECT DATE_TRUNC('month',...) AS month, SUM(...) AS rev FROM orders GROUP BY 1) sub
5. DISTINCT + ORDER BY RANDOM() is illegal. Omit DISTINCT when ordering randomly.
6. Add LIMIT 100 unless the question implies an exact count or total.`

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

// Ask generates SQL, validates it, executes it, and retries once on exec error.
func (c *Client) Ask(ctx context.Context, storeID uuid.UUID, question string) *SQLResult {
	sql, err := c.generateSQL(ctx, question, "")
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

	cols, rows, execErr := executeSQL(ctx, c.readonlyDB, sql, storeID)
	if execErr != nil {
		// Repair loop: feed the error back to the LLM for one retry.
		fixed, repairErr := c.generateSQL(ctx, question, fmt.Sprintf(
			"The previous query failed with: %v\nBad query:\n%s\n\nWrite a corrected query.", execErr, sql,
		))
		if repairErr == nil {
			if valErr := validateSQL(fixed); valErr == nil {
				if c2, r2, e2 := executeSQL(ctx, c.readonlyDB, fixed, storeID); e2 == nil {
					return &SQLResult{
						SQL:         fixed,
						Blocked:     false,
						Columns:     c2,
						Rows:        r2,
						Explanation: fmt.Sprintf("Found %d row(s).", len(r2)),
					}
				}
			}
		}
		// Both attempts failed — return original error.
		return &SQLResult{
			SQL:         sql,
			Blocked:     false,
			Columns:     []string{},
			Rows:        [][]*string{},
			Explanation: fmt.Sprintf("Query execution failed: %v", execErr),
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

// generateSQL calls the LLM. repairHint is empty on first attempt; on retry it
// contains the previous error + bad query so the LLM can correct itself.
func (c *Client) generateSQL(ctx context.Context, question, repairHint string) (string, error) {
	var prompt string
	if repairHint == "" {
		prompt = fmt.Sprintf(
			"You are a PostgreSQL expert for an e-commerce analytics platform.\nGenerate a single SQL SELECT query answering the user's question.\n\nSCHEMA:\n%s\n\n%s\n\nQUESTION: %s",
			sqlSchema, sqlRules, question,
		)
	} else {
		prompt = fmt.Sprintf(
			"You are a PostgreSQL expert for an e-commerce analytics platform.\nFix the SQL query below.\n\nSCHEMA:\n%s\n\n%s\n\nQUESTION: %s\n\n%s",
			sqlSchema, sqlRules, question, repairHint,
		)
	}

	raw, err := c.complete(ctx, prompt, 0.1)
	if err != nil {
		return "", err
	}
	return cleanSQL(raw), nil
}

func cleanSQL(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip markdown fences
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
