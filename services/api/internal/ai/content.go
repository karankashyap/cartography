package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ProductInput struct {
	ID          string
	Title       string
	Description string
	ProductType string
	Vendor      string
	Tags        []string
}

type ContentResult struct {
	ProductID string
	Kind      string
	Content   string
}

type SEOContent struct {
	Title           string   `json:"title"`
	MetaDescription string   `json:"meta_description"`
	Keywords        []string `json:"keywords"`
}

// GenerateContent generates product content for the given kind.
// Returns one ContentResult per product.
func (c *Client) GenerateContent(ctx context.Context, products []ProductInput, kind string) ([]ContentResult, error) {
	results := make([]ContentResult, 0, len(products))
	for _, p := range products {
		content, err := c.generateOne(ctx, p, kind)
		if err != nil {
			// Degrade gracefully per V6: skip failed product, continue batch
			results = append(results, ContentResult{
				ProductID: p.ID,
				Kind:      kind,
				Content:   fmt.Sprintf("Content generation failed: %v", err),
			})
			continue
		}
		results = append(results, ContentResult{
			ProductID: p.ID,
			Kind:      kind,
			Content:   content,
		})
	}
	return results, nil
}

func (c *Client) generateOne(ctx context.Context, p ProductInput, kind string) (string, error) {
	var prompt string
	var temperature float64

	tagStr := strings.Join(p.Tags, ", ")
	productContext := fmt.Sprintf("Title: %s\nType: %s\nVendor: %s\nTags: %s",
		p.Title, p.ProductType, p.Vendor, tagStr)

	switch kind {
	case "DESCRIPTION":
		temperature = 0.7
		prompt = fmt.Sprintf(`Write a compelling product description for an e-commerce listing.

Product details:
%s

Requirements:
- 2-4 sentences
- Highlight key benefits, not just features
- Conversational but professional tone
- No marketing buzzwords or hyperbole
- Output ONLY the description text, no labels or JSON`, productContext)

	case "SEO":
		temperature = 0.3
		prompt = fmt.Sprintf(`Generate SEO metadata for this product listing.

Product details:
%s

Requirements:
- title: max 60 characters, include main keyword
- meta_description: max 155 characters, include a call-to-action
- keywords: 5-8 relevant search terms

Respond with ONLY valid JSON, no other text:
{"title":"...","meta_description":"...","keywords":["...","..."]}`, productContext)

	case "EMAIL":
		temperature = 0.8
		prompt = fmt.Sprintf(`Write a 3-paragraph marketing email to promote this product.

Product details:
%s

Requirements:
- Paragraph 1: Hook — engage with a problem or desire
- Paragraph 2: Solution — explain how this product solves it
- Paragraph 3: CTA — clear call to action with urgency
- Professional, not pushy
- Output ONLY the email body text`, productContext)

	default:
		return "", fmt.Errorf("unknown content kind: %s", kind)
	}

	msgs := []chatMessage{
		{Role: "system", Content: "You are an expert e-commerce copywriter. Be concise and precise. Output only what is requested — no preamble, no labels, no explanation."},
		{Role: "user", Content: prompt},
	}
	raw, err := c.complete(ctx, msgs, temperature, 0)
	if err != nil {
		return "", err
	}

	if kind == "SEO" {
		return normalizeSEO(raw)
	}
	return strings.TrimSpace(raw), nil
}

func normalizeSEO(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var inner []string
		for _, l := range lines {
			if strings.HasPrefix(l, "```") {
				continue
			}
			inner = append(inner, l)
		}
		raw = strings.Join(inner, "\n")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return raw, nil // return raw if not valid JSON — still useful
	}
	raw = raw[start : end+1]

	// Validate it's parseable JSON with expected fields
	var seo SEOContent
	if err := json.Unmarshal([]byte(raw), &seo); err != nil {
		return raw, nil
	}

	out, _ := json.MarshalIndent(seo, "", "  ")
	return string(out), nil
}
