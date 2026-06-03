// Package search_emails searches for emails in a Microsoft Outlook mailbox.
package search_emails

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Emails"
	Description  = "Search for emails in an Outlook mailbox"
	Website      = "https://www.flomation.co"
	Icon         = "magnifying-glass"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,subject,from,receivedDateTime,bodyPreview"
)

var Inputs = [...]core.Connection{
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Query", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "emails", Type: core.ConnectionTypeString, Label: "Emails (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Result Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query := microsoft.OptStr("query", inputs)
	if query == "" {
		return microsoft.ErrorResult("query is required")
	}

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

	endpoint := fmt.Sprintf("%s/me/messages?$search=%s&$top=%d&$select=%s",
		microsoft.GraphAPI, url.QueryEscape("\""+query+"\""), maxResults, selectFields)

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
		lines = append(lines, fmt.Sprintf("- %s (from %s, %s)",
			e.Subject, e.From.EmailAddress.Address, e.ReceivedDateTime))
	}
	summary := fmt.Sprintf("Found %d emails matching \"%s\":\n%s", len(resp.Value), query, strings.Join(lines, "\n"))

	return map[string]interface{}{
		"tool_result": summary,
		"emails":      string(emailsJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
