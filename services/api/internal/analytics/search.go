package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const embeddingModel = "nomic-embed-text"

// SearchProducts performs semantic search via pgvector cosine similarity.
// Falls back to pg_trgm text search if embeddings unavailable. V6: graceful degradation.
func SearchProducts(ctx context.Context, db *pgxpool.Pool, ollamaURL string, storeID uuid.UUID, query string, limit int) ([]Product, error) {
	if limit <= 0 {
		limit = 10
	}

	embedding, err := embedText(ctx, ollamaURL, query)
	if err == nil && len(embedding) > 0 {
		results, err := vectorSearch(ctx, db, storeID, embedding, limit)
		if err == nil {
			return results, nil
		}
	}

	return textSearch(ctx, db, storeID, query, limit)
}

type Product struct {
	ID          string
	Title       string
	Description string
	ProductType string
	Vendor      string
	Tags        []string
}

func vectorSearch(ctx context.Context, db *pgxpool.Pool, storeID uuid.UUID, embedding []float32, limit int) ([]Product, error) {
	vec := pgvecLiteral(embedding)
	rows, err := db.Query(ctx, `
		SELECT id::TEXT, title, COALESCE(description, ''), COALESCE(product_type, ''),
		       COALESCE(vendor, ''), COALESCE(tags, '{}')
		FROM products
		WHERE store_id = $1
		  AND embedding IS NOT NULL
		ORDER BY embedding <=> $2::vector
		LIMIT $3
	`, storeID, vec, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func textSearch(ctx context.Context, db *pgxpool.Pool, storeID uuid.UUID, query string, limit int) ([]Product, error) {
	rows, err := db.Query(ctx, `
		SELECT id::TEXT, title, COALESCE(description, ''), COALESCE(product_type, ''),
		       COALESCE(vendor, ''), COALESCE(tags, '{}')
		FROM products
		WHERE store_id = $1
		  AND (
		      title ILIKE '%' || $2 || '%'
		   OR description ILIKE '%' || $2 || '%'
		   OR $2 = ANY(tags)
		  )
		ORDER BY similarity(title, $2) DESC
		LIMIT $3
	`, storeID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func scanProducts(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]Product, error) {
	var products []Product
	for rows.Next() {
		var p Product
		var tags []string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.ProductType, &p.Vendor, &tags); err != nil {
			return nil, err
		}
		p.Tags = tags
		products = append(products, p)
	}
	return products, rows.Err()
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func embedText(ctx context.Context, ollamaURL string, text string) ([]float32, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	body, _ := json.Marshal(embedRequest{Model: embeddingModel, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result embedResponse
	if err := json.Unmarshal(data, &result); err != nil || len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("embed parse: %w", err)
	}
	return result.Embeddings[0], nil
}

// pgvecLiteral converts []float32 to Postgres vector literal "[0.1,0.2,...]"
func pgvecLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	buf := make([]byte, 0, len(v)*10)
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = fmt.Appendf(buf, "%g", f)
	}
	buf = append(buf, ']')
	return string(buf)
}
