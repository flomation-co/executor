// Package get retrieves metadata for a PowerPoint presentation in Microsoft OneDrive.
package get

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Presentation"
	Description  = "Retrieve metadata for a PowerPoint presentation in OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "display+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,name,size,createdDateTime,lastModifiedDateTime,webUrl,file"
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Presentation Item ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Size (bytes)"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "modified_at", Type: core.ConnectionTypeString, Label: "Modified At"},
	{Name: "presentation", Type: core.ConnectionTypeString, Label: "Full Metadata (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	if itemID == "" {
		return microsoft.ErrorResult("item_id is required")
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

	endpoint := fmt.Sprintf("%s/me/drive/items/%s?$select=%s",
		microsoft.GraphAPI, itemID, selectFields)

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

	var item struct {
		ID                   string `json:"id"`
		Name                 string `json:"name"`
		Size                 int64  `json:"size"`
		LastModifiedDateTime string `json:"lastModifiedDateTime"`
		WebURL               string `json:"webUrl"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	summary := fmt.Sprintf("%s — %d bytes, modified %s", item.Name, item.Size, item.LastModifiedDateTime)

	return map[string]interface{}{
		"tool_result":  summary,
		"name":         item.Name,
		"size":         fmt.Sprintf("%d", item.Size),
		"web_url":      item.WebURL,
		"modified_at":  item.LastModifiedDateTime,
		"presentation": string(body),
		"success":      true,
		"error":        "",
	}, nil
}
