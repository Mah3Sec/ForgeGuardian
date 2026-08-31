package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultOpenAIModel = "gpt-4o"

type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func NewOpenAIProvider(cfg Config) (*OpenAIProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai: API key required (set OPENAI_API_KEY)")
	}
	model := cfg.Model
	if model == "" {
		model = defaultOpenAIModel
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{apiKey: cfg.APIKey, model: model, baseURL: base}, nil
}

func (p *OpenAIProvider) Name() string         { return ProviderOpenAI }
func (p *OpenAIProvider) SupportsToolUse() bool { return true }

type openaiMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []openaiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolDef      `json:"function"`
}

type openaiToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Tools    []openaiTool    `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens,omitempty"`
}

type openaiResponse struct {
	Choices []struct {
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (Response, error) {
	var msgs []openaiMessage

	if req.SystemPrompt != "" {
		msgs = append(msgs, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, openaiMessage{Role: m.Role, Content: m.Content})
	}
	for _, tr := range req.ToolResults {
		msgs = append(msgs, openaiMessage{
			Role:       "tool",
			Content:    tr.Content,
			ToolCallID: tr.ToolCallID,
		})
	}

	body := openaiRequest{
		Model:     p.model,
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
	}

	for _, t := range req.Tools {
		body.Tools = append(body.Tools, openaiTool{
			Type: "function",
			Function: openaiToolDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters: map[string]any{
					"type":       "object",
					"properties": t.Parameters,
					"required":   t.Required,
				},
			},
		})
	}

	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("openai: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("openai: HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return Response{}, fmt.Errorf("openai: parse response: %w", err)
	}
	if oaiResp.Error != nil {
		return Response{}, fmt.Errorf("openai: %s", oaiResp.Error.Message)
	}
	if len(oaiResp.Choices) == 0 {
		return Response{}, fmt.Errorf("openai: empty response")
	}

	choice := oaiResp.Choices[0]
	resp := Response{
		Text: choice.Message.Content,
	}

	if choice.FinishReason == "tool_calls" {
		resp.StopReason = "tool_use"
	} else {
		resp.StopReason = "end_turn"
	}

	for _, tc := range choice.Message.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return resp, nil
}
