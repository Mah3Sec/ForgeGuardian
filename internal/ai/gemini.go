package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultGeminiModel = "gemini-2.5-flash"

type GeminiProvider struct {
	apiKey string
	model  string
}

func NewGeminiProvider(cfg Config) (*GeminiProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini: API key required (set GOOGLE_API_KEY)")
	}
	model := cfg.Model
	if model == "" {
		model = defaultGeminiModel
	}
	return &GeminiProvider{apiKey: cfg.APIKey, model: model}, nil
}

func (p *GeminiProvider) Name() string         { return ProviderGemini }
func (p *GeminiProvider) SupportsToolUse() bool { return true }

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string               `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiToolResult     `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiToolResult struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiToolDecl struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type geminiRequest struct {
	Contents         []geminiContent     `json:"contents"`
	SystemInstruction *geminiContent      `json:"systemInstruction,omitempty"`
	Tools            []geminiToolDecl    `json:"tools,omitempty"`
	GenerationConfig *geminiGenConfig    `json:"generationConfig,omitempty"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *GeminiProvider) Complete(ctx context.Context, req CompletionRequest) (Response, error) {
	body := geminiRequest{
		GenerationConfig: &geminiGenConfig{MaxOutputTokens: req.MaxTokens},
	}

	if req.SystemPrompt != "" {
		body.SystemInstruction = &geminiContent{
			Role:  "user",
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		body.Contents = append(body.Contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	for _, tr := range req.ToolResults {
		body.Contents = append(body.Contents, geminiContent{
			Role: "user",
			Parts: []geminiPart{{
				FunctionResponse: &geminiToolResult{
					Name:     tr.ToolCallID,
					Response: json.RawMessage(fmt.Sprintf(`{"result":%q}`, tr.Content)),
				},
			}},
		})
	}

	if len(req.Tools) > 0 {
		td := geminiToolDecl{}
		for _, t := range req.Tools {
			td.FunctionDeclarations = append(td.FunctionDeclarations, geminiFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters: map[string]any{
					"type":       "object",
					"properties": t.Parameters,
					"required":   t.Required,
				},
			})
		}
		body.Tools = []geminiToolDecl{td}
	}

	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		p.model)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Api-Key", p.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("gemini: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("gemini: HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return Response{}, fmt.Errorf("gemini: parse response: %w", err)
	}
	if gResp.Error != nil {
		return Response{}, fmt.Errorf("gemini: %s", gResp.Error.Message)
	}
	if len(gResp.Candidates) == 0 {
		return Response{}, fmt.Errorf("gemini: empty response")
	}

	candidate := gResp.Candidates[0]
	resp := Response{StopReason: "end_turn"}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			resp.Text = part.Text
		}
		if part.FunctionCall != nil {
			resp.StopReason = "tool_use"
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    part.FunctionCall.Name,
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		}
	}

	return resp, nil
}
