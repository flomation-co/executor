// Package get retrieves metadata and slide list for a Google Slides presentation.
package get

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Presentation"
	Description  = "Get metadata and slides for a Google Slides presentation"
	Website      = "https://www.flomation.co"
	Icon         = "display"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	slidesAPI = "https://slides.googleapis.com/v1/presentations"
)

var Inputs = [...]core.Connection{
	{Name: "presentation_id", Type: core.ConnectionTypeString, Label: "Presentation ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "slides", Type: core.ConnectionTypeString, Label: "Slides (JSON)"},
	{Name: "slide_count", Type: core.ConnectionTypeInteger, Label: "Slide Count"},
	{Name: "presentation", Type: core.ConnectionTypeString, Label: "Full Presentation (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	presID := google.OptStr("presentation_id", inputs)
	if presID == "" {
		return google.ErrorResult("presentation_id is required")
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

	endpoint := fmt.Sprintf("%s/%s", slidesAPI, presID)

	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var pres struct {
		Title  string `json:"title"`
		Slides []struct {
			ObjectID       string `json:"objectId"`
			SlideProperties struct {
				LayoutObjectID string `json:"layoutObjectId"`
			} `json:"slideProperties"`
		} `json:"slides"`
	}
	_ = json.Unmarshal(body, &pres)

	slidesJSON, _ := json.Marshal(pres.Slides)
	slideCount := int64(len(pres.Slides))

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("'%s' — %d slide(s)", pres.Title, slideCount),
		"title":        pres.Title,
		"slides":       string(slidesJSON),
		"slide_count":  slideCount,
		"presentation": string(body),
		"success":      true,
		"error":        "",
	}, nil
}
