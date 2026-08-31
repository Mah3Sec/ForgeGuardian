package ai

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultBedrockModel = "anthropic.claude-sonnet-4-20250514-v1:0"

type BedrockProvider struct {
	model     string
	region    string
	accessKey string
	secretKey string
	token     string
}

func NewBedrockProvider(cfg Config) (*BedrockProvider, error) {
	model := cfg.Model
	if model == "" {
		model = defaultBedrockModel
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	token := os.Getenv("AWS_SESSION_TOKEN")

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("bedrock: AWS credentials required (set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY)")
	}

	return &BedrockProvider{
		model:     model,
		region:    region,
		accessKey: accessKey,
		secretKey: secretKey,
		token:     token,
	}, nil
}

func (p *BedrockProvider) Name() string         { return ProviderBedrock }
func (p *BedrockProvider) SupportsToolUse() bool { return true }

type bedrockMessage struct {
	Role    string              `json:"role"`
	Content []bedrockContent    `json:"content"`
}

type bedrockContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"toolUseId,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type bedrockTool struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

type bedrockToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type bedrockRequest struct {
	Messages        []bedrockMessage `json:"messages"`
	System          []bedrockContent `json:"system,omitempty"`
	InferenceConfig struct {
		MaxTokens int `json:"maxTokens"`
	} `json:"inferenceConfig"`
	ToolConfig *bedrockToolConfig `json:"toolConfig,omitempty"`
}

type bedrockToolConfig struct {
	Tools []bedrockTool `json:"tools"`
}

type bedrockResponse struct {
	Output struct {
		Message bedrockMessage `json:"message"`
	} `json:"output"`
	StopReason string `json:"stopReason"`
}

func (p *BedrockProvider) Complete(ctx context.Context, req CompletionRequest) (Response, error) {
	body := bedrockRequest{}
	body.InferenceConfig.MaxTokens = req.MaxTokens

	if req.SystemPrompt != "" {
		body.System = []bedrockContent{{Type: "text", Text: req.SystemPrompt}}
	}

	for _, m := range req.Messages {
		body.Messages = append(body.Messages, bedrockMessage{
			Role:    m.Role,
			Content: []bedrockContent{{Type: "text", Text: m.Content}},
		})
	}

	if len(req.ToolResults) > 0 {
		var content []bedrockContent
		for _, tr := range req.ToolResults {
			content = append(content, bedrockContent{
				Type:      "toolResult",
				ToolUseID: tr.ToolCallID,
				Content:   tr.Content,
			})
		}
		body.Messages = append(body.Messages, bedrockMessage{
			Role:    "user",
			Content: content,
		})
	}

	if len(req.Tools) > 0 {
		tc := &bedrockToolConfig{}
		for _, t := range req.Tools {
			tc.Tools = append(tc.Tools, bedrockTool{
				ToolSpec: bedrockToolSpec{
					Name:        t.Name,
					Description: t.Description,
					InputSchema: map[string]any{
						"json": map[string]any{
							"type":       "object",
							"properties": t.Parameters,
							"required":   t.Required,
						},
					},
				},
			})
		}
		body.ToolConfig = tc
	}

	payload, _ := json.Marshal(body)
	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse", p.region, p.model)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.signRequest(httpReq, payload)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("bedrock: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("bedrock: HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	var brResp bedrockResponse
	if err := json.Unmarshal(respBody, &brResp); err != nil {
		return Response{}, fmt.Errorf("bedrock: parse response: %w", err)
	}

	resp := Response{}
	if brResp.StopReason == "tool_use" {
		resp.StopReason = "tool_use"
	} else {
		resp.StopReason = "end_turn"
	}

	for _, block := range brResp.Output.Message.Content {
		switch block.Type {
		case "text":
			resp.Text = block.Text
		case "toolUse":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return resp, nil
}

func (p *BedrockProvider) signRequest(req *http.Request, payload []byte) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	service := "bedrock"

	req.Header.Set("X-Amz-Date", amzDate)
	if p.token != "" {
		req.Header.Set("X-Amz-Security-Token", p.token)
	}

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, p.region, service)

	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	if p.token != "" {
		signedHeaders = "content-type;host;x-amz-content-sha256;x-amz-date;x-amz-security-token"
	}

	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"), req.URL.Host, payloadHash, amzDate)
	if p.token != "" {
		canonicalHeaders = fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\nx-amz-security-token:%s\n",
			req.Header.Get("Content-Type"), req.URL.Host, payloadHash, amzDate, p.token)
	}

	canonicalRequest := strings.Join([]string{
		"POST",
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := getSignatureKey(p.secretKey, dateStamp, p.region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
