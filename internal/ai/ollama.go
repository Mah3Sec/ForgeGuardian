package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultOllamaModel = "llama3.1"

type OllamaProvider struct {
	baseURL string
	model   string
}

func NewOllamaProvider(cfg Config) (*OllamaProvider, error) {
	base := cfg.BaseURL
	if base == "" {
		base = "http://localhost:11434"
	}
	base = strings.TrimRight(base, "/")
	model := cfg.Model
	if model == "" {
		model = defaultOllamaModel
	}
	return &OllamaProvider{baseURL: base, model: model}, nil
}

func (p *OllamaProvider) Name() string         { return ProviderOllama }
func (p *OllamaProvider) SupportsToolUse() bool { return true }

type ollamaMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	ToolCalls []ollamaToolCall  `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaToolDef  `json:"function"`
}

type ollamaToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	Error   string        `json:"error,omitempty"`
}

func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (Response, error) {
	var msgs []ollamaMessage

	if req.SystemPrompt != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, ollamaMessage{Role: m.Role, Content: m.Content})
	}
	for _, tr := range req.ToolResults {
		msgs = append(msgs, ollamaMessage{Role: "tool", Content: tr.Content})
	}

	body := ollamaRequest{
		Model:    p.model,
		Messages: msgs,
		Stream:   false,
	}

	if req.MaxTokens > 0 {
		body.Options = &ollamaOptions{NumPredict: req.MaxTokens}
	}

	for _, t := range req.Tools {
		body.Tools = append(body.Tools, ollamaTool{
			Type: "function",
			Function: ollamaToolDef{
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
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: request failed (is Ollama running at %s?): %w", p.baseURL, err)
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("ollama: HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	var oResp ollamaResponse
	if err := json.Unmarshal(respBody, &oResp); err != nil {
		return Response{}, fmt.Errorf("ollama: parse response: %w", err)
	}
	if oResp.Error != "" {
		return Response{}, fmt.Errorf("ollama: %s", oResp.Error)
	}

	resp := Response{
		Text:       oResp.Message.Content,
		StopReason: "end_turn",
	}

	for i, tc := range oResp.Message.ToolCalls {
		resp.StopReason = "tool_use"
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:    fmt.Sprintf("ollama_%d", i),
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	return resp, nil
}
