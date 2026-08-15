// Package reviewer uses Claude to review a generated PatchPlan and assess its risk
// before any changes are applied. It acts as a second AI opinion on the planner's output.
package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mah3sec/forgeguardian/internal/agent/executor"
	"github.com/mah3sec/forgeguardian/internal/agent/planner"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	model     anthropic.Model = "claude-sonnet-4-20250514"
	maxTokens = 1024
)

// Review is the output of a patch review.
type Review struct {
	Approved        bool     `json:"approved"`
	RiskAssessment  string   `json:"risk_assessment"`
	Concerns        []string `json:"concerns"`
	Recommendation  string   `json:"recommendation"` // approve|request_changes|reject
	ConfidenceScore float64  `json:"confidence_score"`
}

// Reviewer performs AI-driven review of patch plans.
type Reviewer struct {
	client *anthropic.Client
}

// New creates a new patch Reviewer.
func New(apiKey string) *Reviewer {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	c := anthropic.NewClient(opts...)
	return &Reviewer{client: &c}
}

// Review evaluates a PatchPlan and executor results, returning a risk assessment.
func (r *Reviewer) Review(ctx context.Context, advisory core.Advisory, plan planner.PatchPlan, results []executor.Result) (Review, error) {
	prompt := buildReviewPrompt(advisory, plan, results)

	msg, err := r.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{
			{Text: reviewerSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return Review{}, fmt.Errorf("reviewer: api call: %w", err)
	}

	var raw string
	for _, block := range msg.Content {
		if tb := block.AsText(); tb.Text != "" {
			raw = tb.Text
			break
		}
	}

	return parseReview(raw, plan)
}

const reviewerSystemPrompt = `You are ForgeGuardian's patch review engine — a senior security engineer reviewing an AI-generated patch plan.

Your role: critically evaluate whether the proposed patch is safe to apply.

Rules:
- Check that upgrades target a version that actually fixes the reported CVEs.
- Flag if the patch could introduce breaking changes.
- Reject plans that auto-apply to critical systems without human review.
- Consider the risk_level declared by the planner.
- Respond ONLY with the JSON schema requested.`

func buildReviewPrompt(advisory core.Advisory, plan planner.PatchPlan, results []executor.Result) string {
	planJSON, _ := json.MarshalIndent(plan, "", "  ")
	resultsJSON, _ := json.MarshalIndent(results, "", "  ")

	return fmt.Sprintf(`Review this patch plan for security and correctness.

## Advisory
- Package: %s@%s (%s)
- Severity: %s
- Advisory: %s
- Recommended Action: %s

## Proposed Patch Plan
%s

## Dry-Run Execution Results
%s

## Required JSON Response
{
  "approved": true|false,
  "risk_assessment": "Brief risk assessment (1-2 sentences)",
  "concerns": ["concern 1", "concern 2"],
  "recommendation": "approve|request_changes|reject",
  "confidence_score": 0.0-1.0
}

Respond with ONLY the JSON object.`,
		advisory.Package.Name, advisory.Package.Version, advisory.Package.Ecosystem,
		advisory.Severity, advisory.Advisory, advisory.RecommendedAction,
		string(planJSON), string(resultsJSON),
	)
}

type reviewResponse struct {
	Approved        bool     `json:"approved"`
	RiskAssessment  string   `json:"risk_assessment"`
	Concerns        []string `json:"concerns"`
	Recommendation  string   `json:"recommendation"`
	ConfidenceScore float64  `json:"confidence_score"`
}

func parseReview(raw string, plan planner.PatchPlan) (Review, error) {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "{"); idx >= 0 {
		raw = raw[idx:]
	}
	if idx := strings.LastIndex(raw, "}"); idx >= 0 {
		raw = raw[:idx+1]
	}

	var resp reviewResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		// Fallback: conservative non-approval
		return Review{
			Approved:        false,
			RiskAssessment:  "Could not parse AI review response — manual review required",
			Concerns:        []string{"review parse error"},
			Recommendation:  "request_changes",
			ConfidenceScore: 0.5,
		}, nil
	}

	return Review(resp), nil
}
