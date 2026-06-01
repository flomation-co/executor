// Package add_slide adds a new slide to a Google Slides presentation.
package add_slide

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
	"github.com/google/uuid"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Add Slide"
	Description  = "Add a new slide to a Google Slides presentation"
	Website      = "https://www.flomation.co"
	Icon         = "display"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	slidesAPI = "https://slides.googleapis.com/v1/presentations"
)

var Inputs = [...]core.Connection{
	{Name: "presentation_id", Type: core.ConnectionTypeString, Label: "Presentation ID", Required: true},
	{
		Name:  "layout",
		Type:  core.ConnectionTypeString,
		Label: "Layout",
		Options: []core.ConnectionOption{
			{Name: "Blank", Value: "BLANK"},
			{Name: "Title", Value: "TITLE"},
			{Name: "Title and Body", Value: "TITLE_AND_BODY"},
			{Name: "Title and Two Columns", Value: "TITLE_AND_TWO_COLUMNS"},
			{Name: "Title Only", Value: "TITLE_ONLY"},
			{Name: "Section Header", Value: "SECTION_HEADER"},
		},
	},
	{Name: "insert_at", Type: core.ConnectionTypeInteger, Label: "Insert Position (0-based)"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "slide_id", Type: core.ConnectionTypeString, Label: "Slide ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	presID := google.OptStr("presentation_id", inputs)
	if presID == "" {
		return google.ErrorResult("presentation_id is required")
	}

	layout := google.OptStr("layout", inputs)
	if layout == "" {
		layout = "BLANK"
	}
	insertAt := google.OptInt("insert_at", inputs, -1)

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

	slideID := uuid.New().String()

	createReq := map[string]interface{}{
		"objectId": slideID,
		"slideLayoutReference": map[string]interface{}{
			"predefinedLayout": layout,
		},
	}
	if insertAt >= 0 {
		createReq["insertionIndex"] = insertAt
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"requests": []map[string]interface{}{
			{"createSlide": createReq},
		},
	})

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", slidesAPI, presID)

	status, body, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, payload)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Added %s slide {id:%s}", layout, slideID),
		"slide_id":    slideID,
		"success":     true,
		"error":       "",
	}, nil
}
