// Package share creates a sharing link for a file or folder in Microsoft OneDrive.
package share

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Sharing Link"
	Description  = "Create a sharing link for a OneDrive file or folder"
	Website      = "https://www.flomation.co"
	Icon         = "folder+share-from-square"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Required: true},
	{Name: "link_type", Type: core.ConnectionTypeString, Label: "Link Type", Options: []core.ConnectionOption{
		{Name: "View", Value: "view"},
		{Name: "Edit", Value: "edit"},
		{Name: "Embed", Value: "embed"},
	}},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Options: []core.ConnectionOption{
		{Name: "Anonymous", Value: "anonymous"},
		{Name: "Organisation", Value: "organization"},
	}},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "share_url", Type: core.ConnectionTypeString, Label: "Share URL"},
	{Name: "share_id", Type: core.ConnectionTypeString, Label: "Share ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	if itemID == "" {
		return microsoft.ErrorResult("item_id is required")
	}

	linkType := microsoft.OptStr("link_type", inputs)
	if linkType == "" {
		linkType = "view"
	}
	scope := microsoft.OptStr("scope", inputs)
	if scope == "" {
		scope = "anonymous"
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

	endpoint := fmt.Sprintf("%s/me/drive/items/%s/createLink", microsoft.GraphAPI, itemID)

	payload := map[string]string{
		"type":  linkType,
		"scope": scope,
	}
	body, _ := json.Marshal(payload)

	status, respBody, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(respBody)))
	}

	var resp struct {
		ID   string `json:"id"`
		Link struct {
			WebURL string `json:"webUrl"`
		} `json:"link"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created %s sharing link (%s scope): %s", linkType, scope, resp.Link.WebURL),
		"share_url":   resp.Link.WebURL,
		"share_id":    resp.ID,
		"success":     true,
		"error":       "",
	}, nil
}
