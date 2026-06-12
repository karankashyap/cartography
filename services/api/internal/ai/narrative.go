package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cartograph/api/internal/analytics"
)

// NarrativeInput wraps pre-computed metrics for narrative generation.
// V3: only metrics JSON passed to LLM — no raw rows ever.
type NarrativeInput struct {
	Metrics  *analytics.Metrics
	From, To string
}

// NarrativeOutput is the structured response from the LLM.
type NarrativeOutput struct {
	Summary    string   `json:"summary"`
	Highlights []string `json:"highlights"`
	Actions    []string `json:"actions"`
}

// GenerateNarrative calls Ollama with only pre-computed metric values.
// V6: returns empty insight (not an error) if Ollama is unavailable.
func (c *Client) GenerateNarrative(ctx context.Context, in NarrativeInput) (*NarrativeOutput, error) {
	metricsJSON, err := json.MarshalIndent(in.Metrics, "", "  ")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`You are an e-commerce analyst. Analyze the following metrics and produce insights.

METRICS (these are the ONLY numbers you may reference — do not invent any):
%s

Date range: %s to %s

Respond with ONLY valid JSON in this exact schema, no other text:
{"summary":"<2-3 sentence overview>","highlights":["<observation 1>","<observation 2>","<observation 3>"],"actions":["<action 1>","<action 2>","<action 3>"]}

Rules:
- summary: 2-3 sentences, reference actual numbers from the metrics above
- highlights: exactly 3 notable observations with specific numbers
- actions: exactly 3 concrete recommended actions
- every number you mention MUST appear in the metrics JSON above`, string(metricsJSON), in.From, in.To)

	raw, err := c.complete(ctx, prompt, 0.3)
	if err != nil {
		// V6: Ollama unavailable — return empty insight, let dashboard still render
		return &NarrativeOutput{
			Summary:    "AI insights unavailable. Ensure Ollama is running.",
			Highlights: []string{},
			Actions:    []string{},
		}, nil
	}

	out, err := parseNarrativeResponse(raw)
	if err != nil {
		return &NarrativeOutput{
			Summary:    "AI response could not be parsed. Raw: " + truncate(raw, 200),
			Highlights: []string{},
			Actions:    []string{},
		}, nil
	}

	return out, nil
}

func parseNarrativeResponse(raw string) (*NarrativeOutput, error) {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fences if the model added them
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

	// Find first { ... } block
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	raw = raw[start : end+1]

	var out NarrativeOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	if out.Summary == "" {
		return nil, fmt.Errorf("empty summary in narrative response")
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
