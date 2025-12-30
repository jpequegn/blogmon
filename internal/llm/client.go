package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type ExtractionResult struct {
	Takeaways  []string `json:"takeaways"`
	References []struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Context string `json:"context"`
	} `json:"references"`
	Topics []string `json:"topics"`
}

func NewClient(baseURL, model string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) BuildExtractionPrompt(title, content string) string {
	return fmt.Sprintf(`Analyze this blog post and extract structured information.

Title: %s

Content:
%s

Return a JSON object with:
1. "takeaways": Array of 3-5 key insights (short sentences)
2. "references": Array of objects with "url", "title", "context" for any links mentioned
3. "topics": Array of 2-4 topic tags (e.g., "golang", "distributed-systems", "performance")

Return ONLY valid JSON, no other text.`, title, content)
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(body))
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Response, nil
}

func (c *Client) ExtractInsights(ctx context.Context, title, content string) (*ExtractionResult, error) {
	prompt := c.BuildExtractionPrompt(title, content)

	response, err := c.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Try to parse JSON from response
	var result ExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Try to find JSON in response
		start := bytes.IndexByte([]byte(response), '{')
		end := bytes.LastIndexByte([]byte(response), '}')
		if start >= 0 && end > start {
			if err := json.Unmarshal([]byte(response[start:end+1]), &result); err != nil {
				return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no JSON found in LLM response")
		}
	}

	return &result, nil
}
