package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Client wraps any OpenAI-compatible chat completion API (Ollama, LM Studio, etc.).
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
	readonlyDB *pgxpool.Pool
}

func NewClient(baseURL, model string, readonlyDB *pgxpool.Pool) *Client {
	return &Client{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 600 * time.Second},
		readonlyDB: readonlyDB,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens,omitempty"` // 0 = omit (no limit)
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *Client) BaseURL() string { return c.baseURL }

// complete sends a fully-constructed message array to the LLM.
// maxTokens = 0 means no cap (suitable for long-form content).
// Callers are responsible for building the correct system/user/assistant sequence.
func (c *Client) complete(ctx context.Context, messages []chatMessage, temperature float64, maxTokens int) (string, error) {
	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		Stream:      false,
		MaxTokens:   maxTokens,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm api %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("parse llm response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from llm")
	}
	return chatResp.Choices[0].Message.Content, nil
}
