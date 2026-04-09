// Package remember is the executor action that writes a memory to the
// agent_memory table via the API's Phase 2a internal endpoint.
//
// Typical flow-author usage: drop this action into an orchestrator flow
// wired from an AI action's structured output, or use it directly when
// a flow needs to persist a deterministic fact it just derived. Extraction
// System Flows (Phase 2d) also use this action as their write target.
package remember

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	log "github.com/sirupsen/logrus"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Remember Fact"
	Description  = "Store a durable fact, preference, or piece of feedback in an agent's memory"
	Website      = "https://www.flomation.co"
	Icon         = "brain"
	Date         = "05/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "${flow.agent_id}",
		Required:    true,
	},
	{
		Name:        "agent_user_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent User ID",
		Placeholder: "${flow.agent_user_id}",
		Required:    false,
	},
	{
		Name:     "scope",
		Type:     core.ConnectionTypeString,
		Label:    "Scope",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "User-specific", Value: "user"},
			{Name: "Agent-global", Value: "global"},
		},
	},
	{
		Name:     "memory_type",
		Type:     core.ConnectionTypeString,
		Label:    "Memory Type",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Preference (auto-pinned)", Value: "preference"},
			{Name: "Feedback (auto-pinned)", Value: "feedback"},
			{Name: "Fact", Value: "fact"},
			{Name: "Relationship", Value: "relationship"},
			{Name: "Task", Value: "task"},
			{Name: "Session summary", Value: "session_summary"},
		},
	},
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Title",
		Placeholder: "Short handle for the memory",
		Required:    true,
	},
	{
		Name:        "body",
		Type:        core.ConnectionTypeString,
		Label:       "Body",
		Placeholder: "The fact itself",
		Required:    true,
	},
	{
		Name:        "pinned",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Pinned (always included in system prompt)",
		Placeholder: "false",
		Required:    false,
	},
	{
		Name:        "confidence",
		Type:        core.ConnectionTypeString,
		Label:       "Confidence (0.0–1.0, defaults 1.0)",
		Placeholder: "1.0",
		Required:    false,
	},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS region for embeddings", Placeholder: "us-east-1"},
}

var Outputs = [...]core.Connection{
	{Name: "memory_id", Type: core.ConnectionTypeString, Label: "Memory ID"},
}

// Execute posts to POST /api/v1/internal/agent/:id/memory. Failures are
// returned as node errors rather than silently swallowed — flows that
// author memories deliberately (as opposed to extraction flows that
// might legitimately decide not to write) want to know if the write
// failed so they can retry or surface the failure to the user.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return nil, err
	}

	scope, err := requiredString("scope", inputs)
	if err != nil {
		return nil, err
	}

	memoryType, err := requiredString("memory_type", inputs)
	if err != nil {
		return nil, err
	}

	title, err := requiredString("title", inputs)
	if err != nil {
		return nil, err
	}

	body, err := requiredString("body", inputs)
	if err != nil {
		return nil, err
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	payload := map[string]interface{}{
		"scope":       scope,
		"memory_type": memoryType,
		"title":       title,
		"body":        body,
	}

	if userID := optionalString("agent_user_id", inputs); userID != "" {
		payload["agent_user_id"] = userID
	}
	if pinned := optionalBool("pinned", inputs); pinned {
		payload["pinned"] = true
	}
	if conf := optionalString("confidence", inputs); conf != "" {
		// Parse as a float via JSON round-trip so we don't have to pull
		// strconv into every action. The API validates the range; we
		// just need to pass it through as a number rather than a string.
		var c float64
		if err := json.Unmarshal([]byte(conf), &c); err == nil {
			payload["confidence"] = c
		}
	}

	// Generate embedding if AWS region is available.
	if region := optionalString("aws_region", inputs); region != "" {
		embedText := title + ": " + body
		vec, err := generateEmbedding(flow.GoContext(), region, embedText)
		if err != nil {
			log.WithError(err).Warn("remember: failed to generate embedding, storing without it")
		} else {
			payload["embedding"] = vec
		}
	}

	return postMemory(flow, ctx, agentID, payload)
}

func generateEmbedding(ctx context.Context, region, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	client := bedrockruntime.NewFromConfig(cfg)
	modelID := "amazon.titan-embed-text-v2:0"

	reqBody, _ := json.Marshal(map[string]interface{}{
		"inputText":  text,
		"dimensions": 1024,
		"normalize":  true,
	})

	contentType := "application/json"
	resp, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &modelID,
		Body:        reqBody,
		ContentType: &contentType,
		Accept:      &contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke Bedrock: %w", err)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	return result.Embedding, nil
}

// --- shared request helper ---

func postMemory(flow *core.Flow, ctx *core.ExecutionContext, agentID string, payload map[string]interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/memory", ctx.APIURL, agentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call remember endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]interface{}{
		"memory_id": result.ID,
	}, nil
}

// --- input helpers ---

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func optionalBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	// Fall through: some flows pass booleans as string literals.
	if s := c.String(); s != nil {
		return *s == "true"
	}
	return false
}
