package ai

import (
	"context"
	"fmt"
	"regexp"
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

const sqlRules = `STRICT RULES:
1. Output ONLY raw SQL — no markdown fences, no comments, no trailing semicolon.
2. GROUP BY: every non-aggregate column in SELECT must appear in GROUP BY.
   ORDER BY must reference GROUP BY expressions or their aliases.
   Use DATE_TRUNC('month', ordered_at) for monthly grouping — never EXTRACT.
3. Window functions and GROUP BY cannot appear at the same query level.
   For running totals: GROUP BY in a subquery, window function in the outer SELECT.
4. Date ranges: ordered_at >= 'YYYY-01-01' AND ordered_at < 'YYYY+1-01-01'
5. DISTINCT + ORDER BY RANDOM() is illegal — omit DISTINCT when ordering randomly.
6. Add LIMIT 100 unless the question asks for a total or exact count.`

var forbiddenKeywords = []string{
	"INSERT ", "UPDATE ", "DELETE ", "DROP ", "CREATE ", "TRUNCATE ",
	"ALTER ", "GRANT ", "REVOKE ", "COPY ", "EXECUTE ", "PERFORM ",
	"CALL ", "DO ", "PG_", "INFORMATION_SCHEMA", "$$",
}

// sqlKeywords that cannot be a table alias.
var sqlKeywords = map[string]bool{
	"ON": true, "WHERE": true, "GROUP": true, "ORDER": true,
	"LIMIT": true, "HAVING": true, "INNER": true, "LEFT": true,
	"RIGHT": true, "CROSS": true, "FULL": true, "JOIN": true,
	"AS": true, "SET": true, "WITH": true, "SELECT": true,
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

// Ask generates SQL, injects store filters, validates, executes, retries once on error.
func (c *Client) Ask(ctx context.Context, storeID uuid.UUID, question string) *SQLResult {
	sid := storeID.String()

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

	// Inject store filter before validation/execution — never trust the LLM for this.
	sql = injectStoreFilters(sql, sid)

	if valErr := validateSQL(sql); valErr != nil {
		return &SQLResult{
			SQL:         sql,
			Blocked:     true,
			BlockReason: valErr.Error(),
			Columns:     []string{},
			Rows:        [][]*string{},
			Explanation: "Query blocked by safety guardrail.",
		}
	}

	cols, rows, execErr := executeSQL(ctx, c.readonlyDB, sql)
	if execErr != nil {
		// Repair loop: feed execution error back to LLM for one retry.
		fixed, repairErr := c.generateSQL(ctx, question, fmt.Sprintf(
			"The previous query failed with: %v\nBad query:\n%s\n\nWrite a corrected query.", execErr, sql,
		))
		if repairErr == nil {
			fixed = injectStoreFilters(fixed, sid)
			if valErr := validateSQL(fixed); valErr == nil {
				if c2, r2, e2 := executeSQL(ctx, c.readonlyDB, fixed); e2 == nil {
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

// injectStoreFilters rewrites every FROM/JOIN reference to orders, products, or
// customers as a filtered subquery. Store isolation is enforced here, never by
// the LLM.
//
// Example:
//
//	FROM orders WHERE ordered_at > '2022-01-01'
//	→ FROM (SELECT * FROM orders WHERE store_id = '<uuid>') AS orders WHERE ordered_at > '2022-01-01'
func injectStoreFilters(sql, storeID string) string {
	for _, table := range []string{"orders", "products", "customers"} {
		// Match: (FROM|JOIN) <table> [AS] [alias]
		re := regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+` + table + `\b(?:\s+(?:AS\s+)?(\w+))?`)
		sql = re.ReplaceAllStringFunc(sql, func(match string) string {
			subs := re.FindStringSubmatch(match)
			keyword := subs[1]
			alias := subs[2]
			if alias == "" || sqlKeywords[strings.ToUpper(alias)] {
				alias = table
			}
			return fmt.Sprintf(`%s (SELECT * FROM %s WHERE store_id = '%s') AS %s`,
				keyword, table, storeID, alias)
		})
	}
	return sql
}

// generateSQL calls the LLM. repairHint is empty on first call; on retry it
// contains the exec error and bad query.
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
	upper := strings.ToUpper(raw)
	if strings.Contains(upper, "SELECT DISTINCT") {
		raw = raw[:strings.Index(upper, "DISTINCT")] + raw[strings.Index(upper, "DISTINCT")+len("DISTINCT "):]
	}
	return raw
}

func validateSQL(sql string) error {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT queries are allowed")
	}
	for _, kw := range forbiddenKeywords {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("forbidden keyword in query: %s", strings.TrimSpace(kw))
		}
	}
	return nil
}

func executeSQL(ctx context.Context, db *pgxpool.Pool, sql string) ([]string, [][]*string, error) {
	upper := strings.ToUpper(sql)
	if !strings.Contains(upper, "LIMIT") {
		sql = sql + " LIMIT 100"
	}

	rows, err := db.Query(ctx, sql)
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
