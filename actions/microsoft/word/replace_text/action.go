// Package replace_text downloads a Word document as HTML, performs text
// replacement, and returns the modified content.
package replace_text

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Replace Text"
	Description  = "Find and replace text in a Word document (via HTML conversion)"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+pencil"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Document Item ID", Required: true},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search Text", Required: true},
	{Name: "replace", Type: core.ConnectionTypeString, Label: "Replace With", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Modified HTML Content"},
	{Name: "replacements", Type: core.ConnectionTypeString, Label: "Replacement Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	if itemID == "" {
		return microsoft.ErrorResult("item_id is required")
	}

	searchConn := core.FindConnection("search", inputs)
	if searchConn == nil || searchConn.String() == nil || *searchConn.String() == "" {
		return microsoft.ErrorResult("search text is required")
	}
	searchText := *searchConn.String()

	replaceConn := core.FindConnection("replace", inputs)
	replaceText := ""
	if replaceConn != nil && replaceConn.String() != nil {
		replaceText = *replaceConn.String()
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

	// Download the document as HTML for text manipulation.
	contentEndpoint := fmt.Sprintf("%s/me/drive/items/%s/content?format=html",
		microsoft.GraphAPI, itemID)

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

	html := string(body)

	// Fetch metadata for a meaningful summary.
	metaEndpoint := fmt.Sprintf("%s/me/drive/items/%s?$select=name",
		microsoft.GraphAPI, itemID)
	var docName string
	metaStatus, metaBody, metaErr := microsoft.DoRequest(flow, "GET", metaEndpoint, token.AccessToken, nil)
	if metaErr == nil && metaStatus >= 200 && metaStatus < 300 {
		var meta struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(metaBody, &meta) == nil {
			docName = meta.Name
		}
	}

	// Count occurrences then perform replacement.
	count := strings.Count(html, searchText)
	modified := strings.ReplaceAll(html, searchText, replaceText)

	summary := fmt.Sprintf("Replaced %d occurrence(s) of %q in %s", count, searchText, docName)

	return map[string]interface{}{
		"tool_result":  summary,
		"content":      modified,
		"replacements": fmt.Sprintf("%d", count),
		"success":      true,
		"error":        "",
	}, nil
}
