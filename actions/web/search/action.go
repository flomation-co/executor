// Package web_search is a tool action that searches the web via the
// Brave Search API. Designed to be wired to an AI action's Tools handle
// so the model can search mid-conversation.
package web_search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Web Search"
	Description  = "Search the web using the Brave Search API"
	Website      = "https://www.flomation.co"
	Icon         = "magnifying-glass"
	Date         = "07/04/2026"
	Type         = core.ActionTypeAction

	braveAPIBase = "https://api.search.brave.com/res/v1/web/search"
)

var Inputs = [...]core.Connection{
	{
		Name:        "query",
		Type:        core.ConnectionTypeString,
		Label:       "Search Query",
		Placeholder: "What to search for",
		Required:    true,
	},
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "Brave API Key",
		Placeholder: "BSA...",
		Required:    true,
	},
	{
		Name:        "count",
		Type:        core.ConnectionTypeInteger,
		Label:       "Number of Results",
		Placeholder: "5",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Search Results (text)"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Raw Results (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queryConn := core.FindConnection("query", inputs)
	if queryConn == nil || queryConn.String() == nil || *queryConn.String() == "" {
		return nil, fmt.Errorf("query is required")
	}
	query := *queryConn.String()

	apiKeyConn := core.FindConnection("api_key", inputs)
	if apiKeyConn == nil || apiKeyConn.String() == nil || *apiKeyConn.String() == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	apiKey := *apiKeyConn.String()

	count := int64(5)
	countConn := core.FindConnection("count", inputs)
	if countConn != nil && countConn.Number() != nil && *countConn.Number() > 0 {
		count = *countConn.Number()
	}
	if count > 20 {
		count = 20
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", count))

	endpoint := fmt.Sprintf("%s?%s", braveAPIBase, params.Encode())
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Search failed: %v", err),
			"results":     nil,
			"success":     false,
			"error":       err.Error(),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Search API returned %d", resp.StatusCode),
			"results":     nil,
			"success":     false,
			"error":       string(respBody),
		}, nil
	}

	var searchResult struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(respBody, &searchResult); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	// Format as readable text for the AI
	var sb strings.Builder
	for i, r := range searchResult.Web.Results {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"results":     searchResult.Web.Results,
		"success":     true,
		"error":       "",
	}, nil
}
