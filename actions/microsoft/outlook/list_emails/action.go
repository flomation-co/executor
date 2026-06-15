// Package list_emails lists emails from a Microsoft Outlook mailbox or folder.
package list_emails

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Emails"
	Description  = "List emails from an Outlook mailbox or folder"
	Website      = "https://www.flomation.co"
	Icon         = "envelopes-bulk"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,subject,from,receivedDateTime,isRead,bodyPreview"
)

var Inputs = [...]core.Connection{
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Folder ID"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "emails", Type: core.ConnectionTypeString, Label: "Emails (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Email Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	folderID := microsoft.OptStr("folder_id", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)
	maxResults := microsoft.OptInt("max_results", inputs, 25)

	tokens, err := microsoft.FetchTokens(flow, credential, "mail_read")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	var endpoint string
	if folderID != "" {
		endpoint = fmt.Sprintf("%s/me/mailFolders/%s/messages?$top=%d&$select=%s",
			microsoft.GraphAPI, folderID, maxResults, selectFields)
	} else {
		endpoint = fmt.Sprintf("%s/me/messages?$top=%d&$select=%s",
			microsoft.GraphAPI, maxResults, selectFields)
	}

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
		Value []struct {
			ID               string `json:"id"`
			Subject          string `json:"subject"`
			ReceivedDateTime string `json:"receivedDateTime"`
			IsRead           bool   `json:"isRead"`
			BodyPreview      string `json:"bodyPreview"`
			From             struct {
				EmailAddress struct {
					Name    string `json:"name"`
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"from"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	emailsJSON, _ := json.Marshal(resp.Value)

	var lines []string
	for _, e := range resp.Value {
		readStatus := "unread"
		if e.IsRead {
			readStatus = "read"
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s (from %s, %s)",
			readStatus, e.Subject, e.From.EmailAddress.Address, e.ReceivedDateTime))
	}
	summary := fmt.Sprintf("Found %d emails:\n%s", len(resp.Value), strings.Join(lines, "\n"))

	return map[string]interface{}{
		"tool_result": summary,
		"emails":      string(emailsJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
