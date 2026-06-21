// Package web_fetch is a tool action that fetches a URL and extracts
// text content. Designed to be wired to an AI action's Tools handle
// so the model can read web pages mid-conversation.
package web_fetch

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Web Fetch"
	Description  = "Fetch a URL and extract the text content"
	Website      = "https://www.flomation.co"
	Icon         = "globe+arrow-down"
	Date         = "07/04/2026"
	Type         = core.ActionTypeAction

	defaultMaxLength = 10000
)

var Inputs = [...]core.Connection{
	{
		Name:        "url",
		Type:        core.ConnectionTypeString,
		Label:       "URL",
		Placeholder: "https://example.com",
		Required:    true,
	},
	{
		Name:        "max_length",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max Content Length",
		Placeholder: "10000",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Page Content (text)"},
	// Mirror of tool_result under the conventional name flow authors expect
	// when wiring this action into a regular (non-AI) flow. tool_result is
	// the canonical AI-callable first output (per CLAUDE.md), but it reads
	// awkwardly as a node input — ${node.content} is the natural form.
	{Name: "content", Type: core.ConnectionTypeString, Label: "Page Content (text)"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Page Title"},
	{Name: "status_code", Type: core.ConnectionTypeInteger, Label: "HTTP Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Tag-stripping regexes for simple HTML-to-text conversion
var (
	reScript    = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle     = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reTag       = regexp.MustCompile(`<[^>]+>`)
	reMultiLine = regexp.MustCompile(`\n{3,}`)
	reTitle     = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	urlConn := core.FindConnection("url", inputs)
	if urlConn == nil || urlConn.String() == nil || *urlConn.String() == "" {
		return nil, fmt.Errorf("url is required")
	}
	targetURL := *urlConn.String()

	maxLen := int64(defaultMaxLength)
	maxLenConn := core.FindConnection("max_length", inputs)
	if maxLenConn != nil && maxLenConn.Number() != nil && *maxLenConn.Number() > 0 {
		maxLen = *maxLenConn.Number()
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, targetURL, nil)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to create request: %v", err),
			"content":     "",
			"title":       "",
			"status_code": 0,
			"success":     false,
			"error":       err.Error(),
		}, nil
	}
	req.Header.Set("User-Agent", "Flomation/1.0 (AI Agent Tool)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to fetch URL: %v", err),
			"content":     "",
			"title":       "",
			"status_code": 0,
			"success":     false,
			"error":       err.Error(),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("HTTP %d from %s", resp.StatusCode, targetURL),
			"content":     "",
			"title":       "",
			"status_code": resp.StatusCode,
			"success":     false,
			"error":       fmt.Sprintf("HTTP %d", resp.StatusCode),
		}, nil
	}

	// Read body with a generous limit (5 MB raw, then truncate text)
	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	html := string(rawBody)

	// Extract title
	title := ""
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		title = strings.TrimSpace(reTag.ReplaceAllString(m[1], ""))
	}

	// Strip scripts, styles, then all tags
	text := reScript.ReplaceAllString(html, "")
	text = reStyle.ReplaceAllString(text, "")
	text = reTag.ReplaceAllString(text, "\n")
	text = reMultiLine.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	// Truncate to max length
	if int64(len(text)) > maxLen {
		text = text[:maxLen] + "\n\n[Content truncated]"
	}

	return map[string]interface{}{
		"tool_result": text,
		"content":     text,
		"title":       title,
		"status_code": resp.StatusCode,
		"success":     true,
		"error":       "",
	}, nil
}
