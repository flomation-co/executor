// Package get_file retrieves metadata for a file or folder in Microsoft OneDrive.
package get_file

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get File Info"
	Description  = "Retrieve metadata for a file or folder in OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,name,size,lastModifiedDateTime,createdDateTime,file,folder,webUrl,parentReference"
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Size (bytes)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "modified_at", Type: core.ConnectionTypeString, Label: "Modified At"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "Full Metadata (JSON)"},
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
		CreatedDateTime      string `json:"createdDateTime"`
		LastModifiedDateTime string `json:"lastModifiedDateTime"`
		WebURL               string `json:"webUrl"`
		File                 *struct {
			MimeType string `json:"mimeType"`
		} `json:"file"`
		Folder *struct {
			ChildCount int `json:"childCount"`
		} `json:"folder"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	mimeType := ""
	itemType := "folder"
	if item.File != nil {
		mimeType = item.File.MimeType
		itemType = "file"
	}

	summary := fmt.Sprintf("%s: %s (%s, %d bytes, modified %s)",
		itemType, item.Name, mimeType, item.Size, item.LastModifiedDateTime)

	return map[string]interface{}{
		"tool_result": summary,
		"name":        item.Name,
		"size":        fmt.Sprintf("%d", item.Size),
		"mime_type":   mimeType,
		"web_url":     item.WebURL,
		"created_at":  item.CreatedDateTime,
		"modified_at": item.LastModifiedDateTime,
		"file":        string(body),
		"success":     true,
		"error":       "",
	}, nil
}
