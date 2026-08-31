package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Message struct {
	Role    string // "user", "assistant"
	Content string
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Required    []string       `json:"required"`
}

type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

type ToolResult struct {
	ToolCallID string
	Content    string
}

type Response struct {
	Text       string
	ToolCalls  []ToolCall
	StopReason string // "end_turn" | "tool_use"
}

type CompletionRequest struct {
	SystemPrompt string
	Messages     []Message
	MaxTokens    int
	Tools        []ToolDef
	ToolResults  []ToolResult
}

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (Response, error)
	Name() string
	SupportsToolUse() bool
}

const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
	ProviderBedrock   = "bedrock"
	ProviderGemini    = "gemini"
	ProviderOllama    = "ollama"
)

type Config struct {
	Provider string // anthropic|openai|bedrock|gemini|ollama
	APIKey   string
	Model    string // optional model override
	BaseURL  string // for ollama or custom endpoints
	Region   string // for bedrock
}

func LoadConfig() Config {
	c := Config{
		Provider: strings.ToLower(os.Getenv("FG_AI_PROVIDER")),
		APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		Model:    os.Getenv("FG_AI_MODEL"),
		BaseURL:  os.Getenv("FG_AI_BASE_URL"),
		Region:   os.Getenv("AWS_REGION"),
	}

	if c.Provider == "" {
		c.Provider = detectProvider(c)
	}

	switch c.Provider {
	case ProviderOpenAI:
		if c.APIKey == "" {
			c.APIKey = os.Getenv("OPENAI_API_KEY")
		}
	case ProviderGemini:
		if c.APIKey == "" {
			c.APIKey = os.Getenv("GOOGLE_API_KEY")
		}
	case ProviderBedrock:
		if c.Region == "" {
			c.Region = "us-east-1"
		}
	case ProviderOllama:
		if c.BaseURL == "" {
			c.BaseURL = "http://localhost:11434"
		}
	}

	return c
}

func detectProvider(c Config) string {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return ProviderOpenAI
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		return ProviderGemini
	}
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" {
		return ProviderBedrock
	}
	if c.BaseURL != "" {
		return ProviderOllama
	}
	return ProviderAnthropic
}

func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case ProviderAnthropic:
		return NewAnthropicProvider(cfg)
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg)
	case ProviderBedrock:
		return NewBedrockProvider(cfg)
	case ProviderGemini:
		return NewGeminiProvider(cfg)
	case ProviderOllama:
		return NewOllamaProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown AI provider %q — supported: anthropic, openai, bedrock, gemini, ollama", cfg.Provider)
	}
}

func NewProviderFromEnv() (Provider, error) {
	return NewProvider(LoadConfig())
}
