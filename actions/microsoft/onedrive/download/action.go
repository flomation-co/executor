// Package download downloads a file from Microsoft OneDrive.
package download

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Download File"
	Description  = "Download a file from OneDrive by item ID"
	Website      = "https://www.flomation.co"
	Icon         = "folder+arrow-down"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content (Base64)"},
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Size (bytes)"},
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

	// First, fetch file metadata to get filename and content type.
	metaEndpoint := fmt.Sprintf("%s/me/drive/items/%s?$select=name,size,file",
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
		Name string `json:"name"`
		Size int64  `json:"size"`
		File *struct {
			MimeType string `json:"mimeType"`
		} `json:"file"`
	}
	if err := json.Unmarshal(metaBody, &meta); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse metadata: %v", err))
	}

	contentType := "application/octet-stream"
	if meta.File != nil && meta.File.MimeType != "" {
		contentType = meta.File.MimeType
	}

	// Download the file content. Graph API returns a 302 redirect which
	// the Go HTTP client follows automatically.
	contentEndpoint := fmt.Sprintf("%s/me/drive/items/%s/content",
		microsoft.GraphAPI, itemID)

	contentStatus, contentBody, err := microsoft.DoRequestLong(flow, "GET", contentEndpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if contentStatus < 200 || contentStatus >= 400 {
		if contentStatus == 401 || contentStatus == 403 {
			microsoft.HandleAuthError(flow, token.Email, contentStatus)
		}
		return microsoft.ErrorResult(fmt.Sprintf("download returned %d: %s", contentStatus, microsoft.TruncateBody(contentBody)))
	}

	encoded := base64.StdEncoding.EncodeToString(contentBody)

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Downloaded %s (%s, %d bytes)", meta.Name, contentType, meta.Size),
		"content":      encoded,
		"filename":     meta.Name,
		"content_type": contentType,
		"size":         fmt.Sprintf("%d", meta.Size),
		"success":      true,
		"error":        "",
	}, nil
}
