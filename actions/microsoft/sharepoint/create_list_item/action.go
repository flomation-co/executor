// Package create_list_item creates a new item in a SharePoint list.
package create_list_item

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create List Item"
	Description  = "Create a new item in a SharePoint list"
	Website      = "https://www.flomation.co"
	Icon         = "globe+plus"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "site_id", Type: core.ConnectionTypeString, Label: "Site ID", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Fields (JSON object)", Required: true, Placeholder: `{"Title":"My Item","Status":"Active"}`},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_SHAREPOINT}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID"},
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
	fieldsStr := microsoft.OptStr("fields", inputs)
	if fieldsStr == "" {
		return microsoft.ErrorResult("fields is required")
	}
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	var fieldsData map[string]interface{}
	if err := json.Unmarshal([]byte(fieldsStr), &fieldsData); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("fields must be valid JSON: %s", err.Error()))
	}

	tokens, err := microsoft.FetchTokens(flow, credential, "sharepoint")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	payload := map[string]interface{}{
		"fields": fieldsData,
	}
	reqBody, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/sites/%s/lists/%s/items",
		microsoft.GraphAPI, siteID, listID)

	status, body, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, reqBody)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(body, &resp)

	itemID := ""
	if id, ok := resp["id"].(string); ok {
		itemID = id
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created item %s in list %s", itemID, listID),
		"item_id":     itemID,
		"success":     true,
		"error":       "",
	}, nil
}
