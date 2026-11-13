package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

// Response contains the result of an LLM generation.
type Response struct {
	OutputText       string  `json:"output_text"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMs        int64   `json:"latency_ms"`
	ModelUsed        string  `json:"model_used"`
}

// Provider represents an LLM execution backend.
type Provider interface {
	Execute(ctx context.Context, model string, promptText string, parameters map[string]string) (*Response, error)
}

// Client implements Provider with support for both live APIs and high-fidelity mock simulation.
type Client struct {
	httpClient *http.Client
	openAIKey  string
	openAIBase string
	useMock    bool
}

// NewClient initializes the LLM provider client.
func NewClient() *Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	useMock := apiKey == "" || os.Getenv("PROMPTLedger_MOCK_LLM") == "true"

	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		openAIKey:  apiKey,
		openAIBase: baseURL,
		useMock:    useMock,
	}
}

// SetMockMode toggles simulation mode.
func (c *Client) SetMockMode(mock bool) {
	c.useMock = mock
}

// Execute runs the prompt against either live endpoint or realistic simulation.
func (c *Client) Execute(ctx context.Context, model string, promptText string, params map[string]string) (*Response, error) {
	start := time.Now()

	if c.useMock || c.openAIKey == "" {
		return c.executeSimulation(ctx, model, promptText, start)
	}

	return c.executeOpenAI(ctx, model, promptText, start)
}

func (c *Client) executeOpenAI(ctx context.Context, model string, promptText string, start time.Time) (*Response, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": promptText},
		},
		"temperature": 0.2,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.openAIBase+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.openAIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider api error status %d: %s", res.StatusCode, string(bodyBytes))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(payload.Choices) == 0 {
		return nil, fmt.Errorf("empty choices from provider")
	}

	latency := time.Since(start).Milliseconds()
	cost := calculateCost(model, payload.Usage.PromptTokens, payload.Usage.CompletionTokens)

	return &Response{
		OutputText:       strings.TrimSpace(payload.Choices[0].Message.Content),
		PromptTokens:     payload.Usage.PromptTokens,
		CompletionTokens: payload.Usage.CompletionTokens,
		TotalTokens:      payload.Usage.TotalTokens,
		CostUSD:          cost,
		LatencyMs:        latency,
		ModelUsed:        model,
	}, nil
}

func (c *Client) executeSimulation(ctx context.Context, model string, promptText string, start time.Time) (*Response, error) {
	// Realistic latency calculation based on model type
	baseLatency := 80
	if strings.Contains(model, "4o-mini") || strings.Contains(model, "haiku") || strings.Contains(model, "flash") {
		baseLatency = 95
	} else if strings.Contains(model, "4o") || strings.Contains(model, "sonnet") {
		baseLatency = 210
	}

	jitter, _ := rand.Int(rand.Reader, big.NewInt(60))
	simulatedDuration := time.Duration(baseLatency+int(jitter.Int64())) * time.Millisecond

	select {
	case <-time.After(simulatedDuration):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	promptTokens := len(strings.Fields(promptText)) * 4 / 3
	if promptTokens < 15 {
		promptTokens = 15
	}

	var outputText string
	var completionTokens int

	lower := strings.ToLower(promptText)

	if strings.Contains(lower, "intent") || strings.Contains(lower, "checkout") {
		// E-commerce classifier response
		intent := "PURCHASE"
		confidence := 0.94
		urgency := "HIGH"

		if strings.Contains(lower, "refund") || strings.Contains(lower, "money back") || strings.Contains(lower, "return") {
			intent = "REFUND"
			urgency = "MED"
		} else if strings.Contains(lower, "broken") || strings.Contains(lower, "wrong") || strings.Contains(lower, "complaint") || strings.Contains(lower, "terrible") {
			intent = "COMPLAINT"
			urgency = "HIGH"
		} else if strings.Contains(lower, "when") || strings.Contains(lower, "how") || strings.Contains(lower, "specs") || strings.Contains(lower, "size") {
			intent = "INQUIRY"
			urgency = "LOW"
		}

		if strings.Contains(model, "flash") {
			outputText = fmt.Sprintf(`{"intent": "%s", "confidence": %.2f, "action": "ROUTE", "sentiment": "NEUTRAL"}`, intent, confidence+0.03)
		} else if strings.Contains(model, "haiku") {
			outputText = fmt.Sprintf(`{"intent": "%s", "confidence": %.2f, "urgency": "%s", "reasoning": "Detected relevant checkout signal"}`, intent, confidence+0.01, urgency)
		} else {
			outputText = fmt.Sprintf(`{"intent": "%s", "confidence": %.2f, "reasoning": "Direct intent match from context"}`, intent, confidence)
		}
	} else if strings.Contains(lower, "tier") || strings.Contains(lower, "support") || strings.Contains(lower, "ticket") {
		tier := "TIER_1_DOCS"
		if strings.Contains(lower, "crash") || strings.Contains(lower, "panic") || strings.Contains(lower, "500") || strings.Contains(lower, "database") {
			tier = "TIER_3_ENGINEERING"
		} else if strings.Contains(lower, "billing") || strings.Contains(lower, "invoice") || strings.Contains(lower, "card") {
			tier = "TIER_2_BILLING"
		}
		outputText = fmt.Sprintf(`{"target_tier": "%s", "severity": "P2", "recommended_doc_ids": ["DOC-204", "DOC-102"], "summary": "Ticket routed based on automated triage"}`, tier)
	} else if strings.Contains(lower, "sql") || strings.Contains(lower, "schema") {
		outputText = `{"sql": "SELECT id, email, created_at FROM users WHERE is_active = true ORDER BY created_at DESC LIMIT 50;", "explanation": "Read-only query retrieving active users", "tables_referenced": ["users"]}`
	} else {
		outputText = `{"status": "success", "result": "Model evaluated input successfully", "confidence": 0.96}`
	}

	completionTokens = len(strings.Fields(outputText)) * 4 / 3
	if completionTokens < 10 {
		completionTokens = 10
	}

	latency := time.Since(start).Milliseconds()
	cost := calculateCost(model, promptTokens, completionTokens)

	return &Response{
		OutputText:       outputText,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		CostUSD:          cost,
		LatencyMs:        latency,
		ModelUsed:        model,
	}, nil
}

func calculateCost(model string, promptTokens, completionTokens int) float64 {
	var inputRate, outputRate float64

	switch {
	case strings.Contains(model, "4o-mini"):
		inputRate = 0.15 / 1000000.0
		outputRate = 0.60 / 1000000.0
	case strings.Contains(model, "haiku"):
		inputRate = 0.25 / 1000000.0
		outputRate = 1.25 / 1000000.0
	case strings.Contains(model, "flash"):
		inputRate = 0.10 / 1000000.0
		outputRate = 0.40 / 1000000.0
	case strings.Contains(model, "4o"):
		inputRate = 2.50 / 1000000.0
		outputRate = 10.00 / 1000000.0
	case strings.Contains(model, "sonnet"):
		inputRate = 3.00 / 1000000.0
		outputRate = 15.00 / 1000000.0
	default:
		inputRate = 0.50 / 1000000.0
		outputRate = 1.50 / 1000000.0
	}

	return (float64(promptTokens) * inputRate) + (float64(completionTokens) * outputRate)
}
