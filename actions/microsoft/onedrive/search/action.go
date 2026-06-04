// Package search searches for files and folders in Microsoft OneDrive.
package search

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Files"
	Description  = "Search for files and folders in OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+magnifying-glass"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,name,size,lastModifiedDateTime,file,folder,webUrl"
)

var Inputs = [...]core.Connection{
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Query", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeString, Label: "Results (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Result Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query := microsoft.OptStr("query", inputs)
	if query == "" {
		return microsoft.ErrorResult("query is required")
	}

	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)
	maxResults := microsoft.OptInt("max_results", inputs, 25)

	tokens, err := microsoft.FetchTokens(flow, credential, "onedrive")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	encodedQuery := url.QueryEscape(query)
	endpoint := fmt.Sprintf("%s/me/drive/root/search(q='%s')?$top=%d&$select=%s",
		microsoft.GraphAPI, encodedQuery, maxResults, selectFields)

	status, body, err := microsoft.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var resp struct {
		Value []struct {
			ID                   string `json:"id"`
			Name                 string `json:"name"`
			Size                 int64  `json:"size"`
			LastModifiedDateTime string `json:"lastModifiedDateTime"`
			WebURL               string `json:"webUrl"`
			File                 *struct {
				MimeType string `json:"mimeType"`
			} `json:"file"`
			Folder *struct {
				ChildCount int `json:"childCount"`
			} `json:"folder"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	resultsJSON, _ := json.Marshal(resp.Value)

	var lines []string
	for _, item := range resp.Value {
		itemType := "file"
		if item.Folder != nil {
			itemType = "folder"
		}
		lines = append(lines, fmt.Sprintf("- %s (%s, %d bytes)", item.Name, itemType, item.Size))
	}
	summary := fmt.Sprintf("Found %d results for '%s':\n%s",
		len(resp.Value), query, strings.Join(lines, "\n"))

	return map[string]interface{}{
		"tool_result": summary,
		"results":     string(resultsJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
