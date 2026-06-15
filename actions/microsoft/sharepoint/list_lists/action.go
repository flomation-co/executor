// Package list_lists retrieves all lists from a SharePoint site.
package list_lists

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Lists"
	Description  = "Retrieve all lists from a SharePoint site"
	Website      = "https://www.flomation.co"
	Icon         = "globe+list"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "site_id", Type: core.ConnectionTypeString, Label: "Site ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_SHAREPOINT}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "lists", Type: core.ConnectionTypeString, Label: "Lists (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "List Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	siteID := microsoft.OptStr("site_id", inputs)
	if siteID == "" {
		return microsoft.ErrorResult("site_id is required")
	}
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

	endpoint := fmt.Sprintf("%s/sites/%s/lists?$select=id,name,displayName,description,webUrl,list",
		microsoft.GraphAPI, siteID)

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

	listsJSON, _ := json.Marshal(resp.Value)

	summary := fmt.Sprintf("Found %d list(s)", len(resp.Value))
	for i, list := range resp.Value {
		name, _ := list["displayName"].(string)
		if name != "" {
			if i == 0 {
				summary += ": "
			} else {
				summary += ", "
			}
			summary += name
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"lists":       string(listsJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
