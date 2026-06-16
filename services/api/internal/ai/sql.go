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
variants(id UUID PK, product_id UUID FK→products, sku TEXT, option_color TEXT, option_size TEXT, option_other TEXT, price_cents BIGINT, cost_cents BIGINT, inventory_qty INT)
customers(id UUID PK, store_id UUID FK→stores, external_id TEXT, email_hash TEXT, country TEXT, first_order_at TIMESTAMPTZ, orders_count INT, total_spent_cents BIGINT)
orders(id UUID PK, store_id UUID FK→stores, external_id TEXT, customer_id UUID FK→customers, ordered_at TIMESTAMPTZ, subtotal_cents BIGINT, total_cents BIGINT, currency TEXT, country TEXT, is_returning BOOL)
order_items(id UUID PK, order_id UUID FK→orders, variant_id UUID FK→variants, quantity INT, unit_price_cents BIGINT, line_total_cents BIGINT)`

const sqlRules = `STRICT RULES:
1. Output ONLY raw PostgreSQL SQL — no markdown fences, no comments, no trailing semicolon.
2. Use only the tables and columns in SCHEMA. Never invent columns or tables.
3. GROUP BY: every non-aggregate SELECT column must appear in GROUP BY.
   ORDER BY must reference GROUP BY expressions or their aliases.
   Use DATE_TRUNC('month', ordered_at) for monthly grouping — never EXTRACT.
4. Window functions and GROUP BY cannot appear at the same query level.
   For running totals: GROUP BY in subquery, window function in outer SELECT.
5. Date ranges: ordered_at >= 'YYYY-01-01' AND ordered_at < 'YYYY+1-01-01'
6. DISTINCT + ORDER BY RANDOM() is illegal — omit DISTINCT when ordering randomly.
7. Add LIMIT 100 unless the question asks for a total or exact count.
8. COUNT(DISTINCT x) OVER (...) is NOT valid PostgreSQL — use COUNT(x) OVER or GROUP BY subquery.`

// sqlSystemPrompt is sent as the system role on every request.
// Schema and rules are here so they appear once at the top of the conversation,
// not repeated in every user message.
var sqlSystemPrompt = fmt.Sprintf(
	"Think briefly. Answer quickly. Output ONLY the raw SQL query — no markdown, no explanation, no preamble.\n\nYou are a PostgreSQL expert for an e-commerce analytics platform.\n\nSCHEMA:\n%s\n\n%s",
	sqlSchema, sqlRules,
)

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

// HistoryTurn is one completed Q&A exchange passed as conversation context.
// Only successful (non-blocked) turns should be included.
type HistoryTurn struct {
	Question string
	SQL      string // the generated SQL that executed successfully
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

// Ask generates SQL for question using history as conversation context.
// History turns are sent as prior user/assistant exchanges so the model can
// handle follow-up questions ("same but for last year", "add UK filter", etc.).
// Store filter injection happens server-side — the LLM never sees or sets store_id.
func (c *Client) Ask(ctx context.Context, storeID uuid.UUID, question string, history []HistoryTurn) *SQLResult {
	sid := storeID.String()

	sql, err := c.generateSQL(ctx, question, history, "")
	if err != nil {
		return &SQLResult{
			Blocked:     true,
			BlockReason: fmt.Sprintf("LLM unavailable: %v", err),
			Columns:     []string{},
			Rows:        [][]*string{},
			Explanation: "Could not connect to AI provider. Check that the selected provider is running and accessible.",
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
		// Repair loop: pass the failed SQL back as context so the model sees
		// its own bad attempt and the exact error before writing a fix.
		fixed, repairErr := c.generateSQL(ctx, question, history,
			fmt.Sprintf("Fix this query. PostgreSQL error: %v\nFailed query:\n%s", execErr, sql),
		)
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

// generateSQL builds the multi-turn message array and calls the LLM.
//
// Message structure:
//
//	system:    schema + rules (once, not repeated)
//	user:      history[0].Question
//	assistant: history[0].SQL
//	...
//	user:      current question
//
// For repair (repairHint non-empty), the failed SQL is appended as an
// additional assistant turn followed by a user "fix this" turn:
//
//	...
//	user:      current question
//	assistant: failed SQL
//	user:      "Fix this query. PostgreSQL error: ..."
func (c *Client) generateSQL(ctx context.Context, question string, history []HistoryTurn, repairHint string) (string, error) {
	msgs := []chatMessage{
		{Role: "system", Content: sqlSystemPrompt},
	}

	// Append up to 5 most recent history turns as user/assistant pairs.
	// Cap prevents prompt explosion for thinking models (each turn adds tokens).
	start := 0
	if len(history) > 5 {
		start = len(history) - 5
	}
	for _, h := range history[start:] {
		msgs = append(msgs,
			chatMessage{Role: "user", Content: h.Question},
			chatMessage{Role: "assistant", Content: h.SQL},
		)
	}

	if repairHint == "" {
		msgs = append(msgs, chatMessage{Role: "user", Content: question})
	} else {
		// Repair: show the bad SQL as our previous response, then ask for a fix.
		// The model sees: "I generated X, it failed with Y, fix it."
		badSQL := extractBadSQL(repairHint)
		msgs = append(msgs,
			chatMessage{Role: "user", Content: question},
			chatMessage{Role: "assistant", Content: badSQL},
			chatMessage{Role: "user", Content: repairHint},
		)
	}

	raw, err := c.complete(ctx, msgs, 0.1, 1024)
	if err != nil {
		return "", err
	}
	return cleanSQL(raw), nil
}

// extractBadSQL pulls the SQL out of the repair hint string.
// The hint format is: "Fix this query. PostgreSQL error: ...\nFailed query:\n<sql>"
func extractBadSQL(hint string) string {
	const marker = "Failed query:\n"
	if idx := strings.Index(hint, marker); idx >= 0 {
		return strings.TrimSpace(hint[idx+len(marker):])
	}
	return ""
}

// injectStoreFilters rewrites every FROM/JOIN reference to orders, products, or
// customers as a filtered subquery. Store isolation is enforced here, never by the LLM.
//
// FROM orders WHERE ordered_at > '2022-01-01'
// → FROM (SELECT * FROM orders WHERE store_id = '<uuid>') AS orders WHERE ordered_at > '2022-01-01'
func injectStoreFilters(sql, storeID string) string {
	for _, table := range []string{"orders", "products", "customers"} {
		// Groups: 1=FROM|JOIN, 2=" AS alias"(full), 3=alias(explicit), 4=" word"(full), 5=word(implicit)
		// Explicit AS alias takes priority; bare word alias checked against sqlKeywords.
		// If the bare word is a keyword (e.g. WHERE) it must be put back in the output.
		re := regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+` + table + `\b(?:(\s+AS\s+(\w+))|(\s+(\w+)))?`)
		sql = re.ReplaceAllStringFunc(sql, func(match string) string {
			subs := re.FindStringSubmatch(match)
			keyword := subs[1]
			var alias, suffix string
			switch {
			case subs[3] != "": // explicit AS alias
				alias = subs[3]
			case subs[5] != "": // bare word — alias or SQL keyword
				if sqlKeywords[strings.ToUpper(subs[5])] {
					alias = table
					suffix = subs[4] // e.g. " WHERE" — put it back
				} else {
					alias = subs[5]
				}
			default:
				alias = table
			}
			return fmt.Sprintf(`%s (SELECT * FROM %s WHERE store_id = '%s') AS %s%s`,
				keyword, table, storeID, alias, suffix)
		})
	}
	return sql
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
	// Strip OVER (...) when GROUP BY is also present — mixing window functions
	// with GROUP BY is illegal in PostgreSQL. Removing OVER converts them to
	// plain aggregates, which produces the correct result when grouping.
	raw = stripWindowClauses(raw)
	return raw
}

// stripWindowClauses removes all OVER (...) clauses when GROUP BY is present.
// Uses paren-balanced scan to handle nested parens inside OVER correctly.
func stripWindowClauses(sql string) string {
	upper := strings.ToUpper(sql)
	if !strings.Contains(upper, "GROUP BY") || !strings.Contains(upper, " OVER ") {
		return sql
	}
	out := make([]byte, 0, len(sql))
	i := 0
	for i < len(sql) {
		rest := strings.ToUpper(sql[i:])
		idx := strings.Index(rest, " OVER (")
		if idx < 0 {
			out = append(out, sql[i:]...)
			break
		}
		out = append(out, sql[i:i+idx]...)
		i += idx + len(" OVER (")
		depth := 1
		for i < len(sql) && depth > 0 {
			switch sql[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			i++
		}
	}
	return string(out)
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
