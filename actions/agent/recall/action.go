// Package recall is the executor action that reads memories from the
// agent_memory table via the API's Phase 2a internal list endpoint.
//
// This is the counterpart to agent/remember: it gives flow authors
// explicit access to a user's memories for cases where the automatic
// system-prompt assembly in Launch isn't enough (e.g. composing a
// summary, showing memories to the user, branching on whether a
// specific memory type exists).
package recall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Recall Memories"
	Description  = "Fetch an agent's memories about a specific user"
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
		Required:    true,
	},
	{
		Name:        "pinned_only",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Pinned only",
		Placeholder: "false",
		Required:    false,
	},
	{
		Name:        "limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "Limit",
		Placeholder: "20",
		Required:    false,
	},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Semantic search query"},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS region for embeddings", Placeholder: "us-east-1"},
}

var Outputs = [...]core.Connection{
	{Name: "memories", Type: core.ConnectionTypeObject, Label: "Memories"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return nil, err
	}
	agentUserID, err := requiredString("agent_user_id", inputs)
	if err != nil {
		return nil, err
	}

	execCtx := flow.GetContext()
	if execCtx == nil || execCtx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	query := optionalString("query", inputs)

	// Semantic search path: embed the query and POST to the search endpoint.
	if strings.TrimSpace(query) != "" {
		return semanticRecall(flow, execCtx, agentID, agentUserID, query, optionalInt("limit", inputs), inputs)
	}

	// List path: standard list-memories endpoint.
	q := url.Values{}
	q.Set("agent_user_id", agentUserID)
	if optionalBool("pinned_only", inputs) {
		q.Set("pinned", "true")
	}
	if limit := optionalInt("limit", inputs); limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/memory?%s",
		execCtx.APIURL, agentID, q.Encode())

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call recall endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var memories []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&memories); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]interface{}{
		"memories": memories,
		"count":    len(memories),
	}, nil
}

func semanticRecall(flow *core.Flow, execCtx *core.ExecutionContext, agentID, agentUserID, query string, topK int, inputs []*core.Connection) (map[string]interface{}, error) {
	region := optionalString("aws_region", inputs)
	if region == "" {
		region = "us-east-1"
	}
	if topK <= 0 {
		topK = 10
	}

	vec, err := generateEmbedding(flow.GoContext(), region, query)
	if err != nil {
		log.WithError(err).Warn("recall: failed to generate embedding, falling back to list")
		return map[string]interface{}{
			"memories": []map[string]interface{}{},
			"count":    0,
		}, nil
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"agent_user_id":  agentUserID,
		"embedding":      vec,
		"top_k":          topK,
		"exclude_pinned": false,
	})

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/memory/search", execCtx.APIURL, agentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call search endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var memories []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&memories); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return map[string]interface{}{
		"memories": memories,
		"count":    len(memories),
	}, nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

// generateEmbedding calls AWS Bedrock Titan Embeddings v2 to produce a
// 1024-dimensional vector for semantic search.
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

	resp, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &modelID,
		Body:        reqBody,
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
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

func strPtr(s string) *string { return &s }

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func optionalBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	if s := c.String(); s != nil {
		return *s == "true"
	}
	return false
}

func optionalInt(name string, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return 0
	}
	if i := c.Number(); i != nil {
		return int(*i)
	}
	if s := c.String(); s != nil {
		if n, err := strconv.Atoi(*s); err == nil {
			return n
		}
	}
	return 0
}
