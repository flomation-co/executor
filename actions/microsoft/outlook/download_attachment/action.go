// Package download_attachment downloads an attachment from a Microsoft Outlook email.
package download_attachment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Download Attachment"
	Description  = "Download an attachment from an Outlook email"
	Website      = "https://www.flomation.co"
	Icon         = "file-arrow-down"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID", Required: true},
	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
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
	messageID := microsoft.OptStr("message_id", inputs)
	if messageID == "" {
		return microsoft.ErrorResult("message_id is required")
	}
	attachmentID := microsoft.OptStr("attachment_id", inputs)
	if attachmentID == "" {
		return microsoft.ErrorResult("attachment_id is required")
	}

	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "mail_read")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	endpoint := fmt.Sprintf("%s/me/messages/%s/attachments/%s", microsoft.GraphAPI, messageID, attachmentID)

	status, body, err := microsoft.DoRequestLong(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var attachment struct {
		Name         string `json:"name"`
		ContentType  string `json:"contentType"`
		Size         int    `json:"size"`
		ContentBytes string `json:"contentBytes"`
	}
	if err := json.Unmarshal(body, &attachment); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse attachment: %v", err))
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Downloaded %s (%s, %d bytes)", attachment.Name, attachment.ContentType, attachment.Size),
		"content":      attachment.ContentBytes,
		"filename":     attachment.Name,
		"content_type": attachment.ContentType,
		"size":         fmt.Sprintf("%d", attachment.Size),
		"success":      true,
		"error":        "",
	}, nil
}
