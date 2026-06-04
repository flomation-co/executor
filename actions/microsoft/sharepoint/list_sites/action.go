// Package list_sites searches and lists SharePoint sites.
package list_sites

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Sites"
	Description  = "Search and list SharePoint sites"
	Website      = "https://www.flomation.co"
	Icon         = "globe+list"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Query", Placeholder: "Search term (leave empty for all)"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Max Results"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_SHAREPOINT}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "sites", Type: core.ConnectionTypeString, Label: "Sites (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Site Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query := microsoft.OptStr("query", inputs)
	maxResults := microsoft.OptInt("max_results", inputs, 25)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "sharepoint")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	search := "*"
	if query != "" {
		search = query
	}

	endpoint := fmt.Sprintf("%s/sites?search=%s&$top=%d&$select=id,name,displayName,webUrl",
		microsoft.GraphAPI, url.QueryEscape(search), maxResults)

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
		Value []map[string]interface{} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %s", err.Error()))
	}

	sitesJSON, _ := json.Marshal(resp.Value)

	summary := fmt.Sprintf("Found %d site(s)", len(resp.Value))
	for i, site := range resp.Value {
		name, _ := site["displayName"].(string)
		webURL, _ := site["webUrl"].(string)
		if name != "" {
			if i == 0 {
				summary += ": "
			} else {
				summary += ", "
			}
			summary += name
			if webURL != "" {
				summary += " (" + webURL + ")"
			}
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"sites":       string(sitesJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
