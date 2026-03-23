package http_request

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "HTTP Request"
	Description  = "Send an HTTP request and capture the response"
	Website      = "https://www.flomation.co"
	Icon         = "globe"
	Date         = "23/03/2026"
	Type         = core.ActionTypeAction

	maxResponseBody = 1 << 20 // 1 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "method",
		Type:        core.ConnectionTypeString,
		Label:       "HTTP Method",
		Placeholder: "GET",
		Required:    true,
		Options: []core.ConnectionOption{
			{Name: "GET", Value: "GET"},
			{Name: "POST", Value: "POST"},
			{Name: "PUT", Value: "PUT"},
			{Name: "PATCH", Value: "PATCH"},
			{Name: "DELETE", Value: "DELETE"},
			{Name: "HEAD", Value: "HEAD"},
			{Name: "OPTIONS", Value: "OPTIONS"},
		},
	},
	{
		Name:        "url",
		Type:        core.ConnectionTypeString,
		Label:       "URL",
		Placeholder: "https://example.com/api/v1/resource",
		Required:    true,
	},
	{
		Name:        "headers",
		Type:        core.ConnectionTypeText,
		Label:       "Headers",
		Placeholder: "Content-Type: application/json\nAuthorization: Bearer ...",
	},
	{
		Name:        "body",
		Type:        core.ConnectionTypeText,
		Label:       "Request Body",
		Placeholder: "",
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Timeout (seconds)",
		Placeholder: "30",
	},
}

var Outputs = [...]core.Connection{
	{Name: "status_code", Type: core.ConnectionTypeInteger},
	{Name: "response_body", Type: core.ConnectionTypeString},
	{Name: "response_headers", Type: core.ConnectionTypeString},
	{Name: "success", Type: core.ConnectionTypeBoolean},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	methodConn := core.FindConnection("method", inputs)
	if methodConn == nil || methodConn.String() == nil || *methodConn.String() == "" {
		return nil, fmt.Errorf("HTTP method is required")
	}
	method := strings.ToUpper(*methodConn.String())

	urlConn := core.FindConnection("url", inputs)
	if urlConn == nil || urlConn.String() == nil || *urlConn.String() == "" {
		return nil, fmt.Errorf("URL is required")
	}
	url := *urlConn.String()

	// Parse timeout
	timeoutSecs := int64(30)
	if tc := core.FindConnection("timeout_seconds", inputs); tc != nil && tc.Number() != nil {
		timeoutSecs = *tc.Number()
		if timeoutSecs <= 0 {
			timeoutSecs = 30
		}
		if timeoutSecs > 300 {
			timeoutSecs = 300
		}
	}

	// Build request body
	var bodyReader io.Reader
	if bodyConn := core.FindConnection("body", inputs); bodyConn != nil && bodyConn.String() != nil && *bodyConn.String() != "" {
		bodyReader = strings.NewReader(*bodyConn.String())
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Parse headers (one per line, "Key: Value" format)
	if headersConn := core.FindConnection("headers", inputs); headersConn != nil && headersConn.String() != nil {
		for _, line := range strings.Split(*headersConn.String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutSecs) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, maxResponseBody)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Collect response headers
	var headerLines []string
	for key, values := range resp.Header {
		for _, v := range values {
			headerLines = append(headerLines, key+": "+v)
		}
	}

	return map[string]interface{}{
		"status_code":      resp.StatusCode,
		"response_body":    string(respBody),
		"response_headers": strings.Join(headerLines, "\n"),
		"success":          resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}
