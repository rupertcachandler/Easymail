// Package ai is a minimal OpenAI-compatible chat client for BOI's AI
// features (summarize, draft reply). The provider is user-configured in
// Settings (default: Cheaper Inference); every call is stateless and the
// API key lives only in the local store.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the OpenAI-compatible endpoint used when the user
// doesn't override it.
const DefaultBaseURL = "https://api.cheaperinference.com/v1"

// Client is a tiny OpenAI-compatible API client.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New builds a client; empty baseURL falls back to DefaultBaseURL.
func New(baseURL, apiKey string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &Client{BaseURL: baseURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 120 * time.Second}}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model      string        `json:"model"`
	Messages   []chatMessage `json:"messages"`
	MaxTokens  int           `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error,omitempty"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Error *apiError `json:"error,omitempty"`
}

// Chat sends a system+user prompt and returns the assistant's text.
func (c *Client) Chat(ctx context.Context, model, system, user string, maxTokens int) (string, error) {
	if model == "" {
		return "", fmt.Errorf("no AI model configured — pick one in Settings → AI")
	}
	body, _ := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		if msg := apiErrorMessage(data); msg != "" {
			return "", fmt.Errorf("AI error (HTTP %d): %s", resp.StatusCode, msg)
		}
		return "", fmt.Errorf("AI error: HTTP %d", resp.StatusCode)
	}
	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return "", fmt.Errorf("AI response parse: %w", err)
	}
	if len(cr.Choices) == 0 || strings.TrimSpace(cr.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI response had no content")
	}
	return strings.TrimSpace(cr.Choices[0].Message.Content), nil
}

// ListModels returns the model ids visible to this key. One call both
// validates the key and populates the model picker.
func (c *Client) ListModels() ([]string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model list failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		if msg := apiErrorMessage(data); msg != "" {
			return nil, fmt.Errorf("key check failed (HTTP %d): %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("key check failed: HTTP %d", resp.StatusCode)
	}
	var mr modelsResponse
	if err := json.Unmarshal(data, &mr); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(mr.Data))
	for _, m := range mr.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

// apiErrorMessage extracts the provider's error message from a JSON body,
// falling back to a raw snippet.
func apiErrorMessage(data []byte) string {
	var er struct {
		Error *apiError `json:"error"`
	}
	if json.Unmarshal(data, &er) == nil && er.Error != nil && er.Error.Message != "" {
		return er.Error.Message
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
