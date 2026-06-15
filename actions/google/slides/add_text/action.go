// Package add_text inserts text into a shape on a Google Slides slide.
package add_text

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Add Text to Slide"
	Description  = "Insert text into a slide shape"
	Website      = "https://www.flomation.co"
	Icon         = "display+pencil"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	slidesAPI = "https://slides.googleapis.com/v1/presentations"
)

var Inputs = [...]core.Connection{
	{Name: "presentation_id", Type: core.ConnectionTypeString, Label: "Presentation ID", Required: true},
	{Name: "slide_id", Type: core.ConnectionTypeString, Label: "Slide/Shape ID", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	presID := google.OptStr("presentation_id", inputs)
	if presID == "" {
		return google.ErrorResult("presentation_id is required")
	}
	shapeID := google.OptStr("slide_id", inputs)
	if shapeID == "" {
		return google.ErrorResult("slide_id (shape ID) is required")
	}
	text := google.OptStr("text", inputs)
	if text == "" {
		return google.ErrorResult("text is required")
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
		"requests": []map[string]interface{}{
			{
				"insertText": map[string]interface{}{
					"objectId":    shapeID,
					"text":        text,
					"insertionIndex": 0,
				},
			},
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
		"tool_result": fmt.Sprintf("Inserted text into shape %s", shapeID),
		"success":     true,
		"error":       "",
	}, nil
}
