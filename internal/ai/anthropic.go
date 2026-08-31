package ai

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultAnthropicModel = "claude-sonnet-4-20250514"

type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

func NewAnthropicProvider(cfg Config) (*AnthropicProvider, error) {
	var opts []option.RequestOption
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	c := anthropic.NewClient(opts...)
	model := cfg.Model
	if model == "" {
		model = defaultAnthropicModel
	}
	return &AnthropicProvider{client: &c, model: model}, nil
}

func (p *AnthropicProvider) Name() string         { return ProviderAnthropic }
func (p *AnthropicProvider) SupportsToolUse() bool { return true }

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (Response, error) {
	msgs := buildAnthropicMessages(req)
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  msgs,
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}

	if len(req.Tools) > 0 {
		params.Tools = buildAnthropicTools(req.Tools)
		params.ToolChoice = anthropic.ToolChoiceUnionParam{
			OfAuto: &anthropic.ToolChoiceAutoParam{},
		}
	}

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: %w", err)
	}

	return parseAnthropicResponse(msg), nil
}

func buildAnthropicMessages(req CompletionRequest) []anthropic.MessageParam {
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	if len(req.ToolResults) > 0 {
		var blocks []anthropic.ContentBlockParamUnion
		for _, tr := range req.ToolResults {
			blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolCallID, tr.Content, false))
		}
		msgs = append(msgs, anthropic.NewUserMessage(blocks...))
	}

	return msgs
}

func buildAnthropicTools(tools []ToolDef) []anthropic.ToolUnionParam {
	var out []anthropic.ToolUnionParam
	for _, t := range tools {
		tool := anthropic.ToolUnionParamOfTool(
			anthropic.ToolInputSchemaParam{
				Properties: t.Parameters,
				Required:   t.Required,
			},
			t.Name,
		)
		tool.OfTool.Description = anthropic.String(t.Description)
		out = append(out, tool)
	}
	return out
}

func parseAnthropicResponse(msg *anthropic.Message) Response {
	resp := Response{}

	switch msg.StopReason {
	case anthropic.StopReasonToolUse:
		resp.StopReason = "tool_use"
	default:
		resp.StopReason = "end_turn"
	}

	for _, block := range msg.Content {
		if tb := block.AsText(); tb.Text != "" {
			resp.Text = tb.Text
		}
		if tu := block.AsToolUse(); tu.ID != "" {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    tu.ID,
				Name:  tu.Name,
				Input: tu.Input,
			})
		}
	}

	return resp
}
