// Package get_list_items retrieves items from a SharePoint list with field expansion.
package get_list_items

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get List Items"
	Description  = "Retrieve items from a SharePoint list"
	Website      = "https://www.flomation.co"
	Icon         = "globe+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "site_id", Type: core.ConnectionTypeString, Label: "Site ID", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List ID", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Max Results"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_SHAREPOINT}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "items", Type: core.ConnectionTypeString, Label: "Items (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Item Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	siteID := microsoft.OptStr("site_id", inputs)
	if siteID == "" {
		return microsoft.ErrorResult("site_id is required")
	}
	listID := microsoft.OptStr("list_id", inputs)
	if listID == "" {
		return microsoft.ErrorResult("list_id is required")
	}
	maxResults := microsoft.OptInt("max_results", inputs, 50)
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

	endpoint := fmt.Sprintf("%s/sites/%s/lists/%s/items?$expand=fields&$top=%d",
		microsoft.GraphAPI, siteID, listID, maxResults)

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

	itemsJSON, _ := json.Marshal(resp.Value)

	summary := fmt.Sprintf("Retrieved %d item(s) from list %s", len(resp.Value), listID)

	return map[string]interface{}{
		"tool_result": summary,
		"items":       string(itemsJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
