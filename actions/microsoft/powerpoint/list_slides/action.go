// Package list_slides retrieves metadata and a preview URL for a PowerPoint presentation.
package list_slides

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Preview Presentation"
	Description  = "Get a preview URL and metadata for a PowerPoint presentation"
	Website      = "https://www.flomation.co"
	Icon         = "display+list"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Presentation Item ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "preview_url", Type: core.ConnectionTypeString, Label: "Preview URL"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
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

	// Fetch metadata for name and web URL.
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

	// Request an embeddable preview URL.
	previewEndpoint := fmt.Sprintf("%s/me/drive/items/%s/preview",
		microsoft.GraphAPI, itemID)

	previewStatus, previewBody, err := microsoft.DoRequest(flow, "POST", previewEndpoint, token.AccessToken, []byte("{}"))
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}

	previewURL := ""
	if previewStatus >= 200 && previewStatus < 300 {
		var preview struct {
			GetURL string `json:"getUrl"`
		}
		if json.Unmarshal(previewBody, &preview) == nil {
			previewURL = preview.GetURL
		}
	}

	summary := fmt.Sprintf("Presentation: %s", meta.Name)
	if previewURL != "" {
		summary += " (preview available)"
	}

	return map[string]interface{}{
		"tool_result": summary,
		"preview_url": previewURL,
		"name":        meta.Name,
		"web_url":     meta.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
