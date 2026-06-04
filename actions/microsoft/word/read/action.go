// Package read downloads a Word document from OneDrive in a readable format.
package read

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Document"
	Description  = "Download a Word document as HTML or PDF from OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Document Item ID", Required: true},
	{Name: "format", Type: core.ConnectionTypeString, Label: "Output Format", Placeholder: "html", Options: []core.ConnectionOption{
		{Name: "HTML", Value: "html"},
		{Name: "PDF", Value: "pdf"},
	}},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Document Title"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	if itemID == "" {
		return microsoft.ErrorResult("item_id is required")
	}

	format := microsoft.OptStr("format", inputs)
	if format == "" {
		format = "html"
	}

	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "onedrive")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	// Fetch metadata for title and web URL.
	metaEndpoint := fmt.Sprintf("%s/me/drive/items/%s?$select=name,webUrl",
		microsoft.GraphAPI, itemID)

	metaStatus, metaBody, err := microsoft.DoRequest(flow, "GET", metaEndpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if metaStatus < 200 || metaStatus >= 300 {
		if metaStatus == 401 || metaStatus == 403 {
			microsoft.HandleAuthError(flow, token.Email, metaStatus)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", metaStatus, microsoft.TruncateBody(metaBody)))
	}

	var meta struct {
		Name   string `json:"name"`
		WebURL string `json:"webUrl"`
	}
	if err := json.Unmarshal(metaBody, &meta); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse metadata: %v", err))
	}

	// Download content in the requested format via the conversion endpoint.
	contentEndpoint := fmt.Sprintf("%s/me/drive/items/%s/content?format=%s",
		microsoft.GraphAPI, itemID, format)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, contentEndpoint, nil)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("download request failed: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			microsoft.HandleAuthError(flow, token.Email, resp.StatusCode)
		}
		return microsoft.ErrorResult(fmt.Sprintf("download returned %d: %s", resp.StatusCode, microsoft.TruncateBody(body)))
	}

	var content string
	if format == "pdf" {
		content = base64.StdEncoding.EncodeToString(body)
	} else {
		content = string(body)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Read document %s as %s (%d bytes)", meta.Name, format, len(body)),
		"content":     content,
		"title":       meta.Name,
		"web_url":     meta.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
