// Package move_email moves a Microsoft Outlook email to a different folder.
package move_email

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Move Email"
	Description  = "Move an Outlook email to a different folder"
	Website      = "https://www.flomation.co"
	Icon         = "folder-open"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID", Required: true},
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Destination Folder ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	messageID := microsoft.OptStr("message_id", inputs)
	if messageID == "" {
		return microsoft.ErrorResult("message_id is required")
	}
	folderID := microsoft.OptStr("folder_id", inputs)
	if folderID == "" {
		return microsoft.ErrorResult("folder_id is required")
	}

	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "mail_send")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	payload := map[string]interface{}{
		"destinationId": folderID,
	}
	reqBody, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/me/messages/%s/move", microsoft.GraphAPI, messageID)

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

	return map[string]interface{}{
		"tool_result": "Email moved successfully",
		"success":     true,
		"error":       "",
	}, nil
}
