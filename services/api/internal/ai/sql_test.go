package ai

import (
	"testing"
)

func TestValidateSQL(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "valid select with $1",
			sql:     "SELECT title, price_cents FROM products WHERE store_id = $1",
			wantErr: false,
		},
		{
			name:    "valid with join and $1",
			sql:     "SELECT o.total_cents FROM orders o JOIN products p ON p.store_id = $1 WHERE o.store_id = $1",
			wantErr: false,
		},
		{
			name:    "no $1 parameter",
			sql:     "SELECT * FROM products",
			wantErr: true,
		},
		{
			name:    "UPDATE blocked",
			sql:     "UPDATE products SET title = 'x' WHERE store_id = $1",
			wantErr: true,
		},
		{
			name:    "DELETE blocked",
			sql:     "DELETE FROM products WHERE store_id = $1",
			wantErr: true,
		},
		{
			name:    "DROP blocked",
			sql:     "DROP TABLE products",
			wantErr: true,
		},
		{
			name:    "INSERT blocked",
			sql:     "INSERT INTO products (store_id) VALUES ($1)",
			wantErr: true,
		},
		{
			name:    "pg_catalog blocked",
			sql:     "SELECT * FROM pg_catalog.pg_tables WHERE store_id = $1",
			wantErr: true,
		},
		{
			name:    "information_schema blocked",
			sql:     "SELECT * FROM information_schema.tables WHERE store_id = $1",
			wantErr: true,
		},
		{
			name:    "not starting with SELECT",
			sql:     "WITH cte AS (SELECT 1) DROP TABLE stores",
			wantErr: true,
		},
		{
			name:    "double-dollar blocked",
			sql:     "SELECT $$ || 'x' FROM products WHERE store_id = $1",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSQL(tc.sql)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateSQL(%q) error = %v, wantErr %v", tc.sql, err, tc.wantErr)
			}
		})
	}
}

func TestCleanSQL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fences",
			input: "SELECT 1",
			want:  "SELECT 1",
		},
		{
			name:  "strip sql fence",
			input: "```sql\nSELECT 1\n```",
			want:  "SELECT 1",
		},
		{
			name:  "strip plain fence",
			input: "```\nSELECT 1\n```",
			want:  "SELECT 1",
		},
		{
			name:  "strip trailing semicolon",
			input: "SELECT 1;",
			want:  "SELECT 1",
		},
		{
			name:  "strip leading/trailing whitespace",
			input: "  SELECT 1  ",
			want:  "SELECT 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanSQL(tc.input)
			if got != tc.want {
				t.Errorf("cleanSQL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
