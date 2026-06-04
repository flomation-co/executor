// Package create creates a new Google Slides presentation.
package create

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Presentation"
	Description  = "Create a new Google Slides presentation"
	Website      = "https://www.flomation.co"
	Icon         = "display+plus"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	slidesAPI = "https://slides.googleapis.com/v1/presentations"
)

var Inputs = [...]core.Connection{
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true, Placeholder: "My Presentation"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "presentation_id", Type: core.ConnectionTypeString, Label: "Presentation ID"},
	{Name: "presentation_url", Type: core.ConnectionTypeString, Label: "Presentation URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	title := google.OptStr("title", inputs)
	if title == "" {
		return google.ErrorResult("title is required")
	}

	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}
	token := active[0]

	payload, _ := json.Marshal(map[string]interface{}{
		"title": title,
	})

	status, body, err := google.DoRequest(flow, "POST", slidesAPI, token.AccessToken, payload)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var resp struct {
		PresentationID string `json:"presentationId"`
	}
	_ = json.Unmarshal(body, &resp)

	presURL := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", resp.PresentationID)

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Created presentation '%s' {id:%s}", title, resp.PresentationID),
		"presentation_id":  resp.PresentationID,
		"presentation_url": presURL,
		"success":          true,
		"error":            "",
	}, nil
}
