package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	APIKey, Model string
	HTTP          *http.Client
	BaseURL       string
}

func (c Client) Generate(ctx context.Context, prompt string) (string, error) {
	if c.APIKey == "" || c.Model == "" || strings.TrimSpace(prompt) == "" {
		return "", errors.New("gemini api key, model, and prompt are required")
	}
	base := c.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta/models"
	}
	body, _ := json.Marshal(map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]string{"text": prompt}}}}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/"+url.PathEscape(c.Model)+":generateContent", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.APIKey)
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("gemini status %d", res.StatusCode)
	}
	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal(payload, &decoded) != nil || len(decoded.Candidates) == 0 {
		return "", errors.New("gemini response has no candidate")
	}
	parts := []string{}
	for _, part := range decoded.Candidates[0].Content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	if len(parts) == 0 {
		return "", errors.New("gemini response has no text")
	}
	return strings.Join(parts, "\n"), nil
}
