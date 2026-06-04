// Package get_slide retrieves the content of a specific slide.
package get_slide

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Slide"
	Description  = "Get the content and elements of a specific slide"
	Website      = "https://www.flomation.co"
	Icon         = "display+eye"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	slidesAPI = "https://slides.googleapis.com/v1/presentations"
)

var Inputs = [...]core.Connection{
	{Name: "presentation_id", Type: core.ConnectionTypeString, Label: "Presentation ID", Required: true},
	{Name: "slide_id", Type: core.ConnectionTypeString, Label: "Slide ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "slide", Type: core.ConnectionTypeString, Label: "Slide (JSON)"},
	{Name: "elements", Type: core.ConnectionTypeString, Label: "Page Elements (JSON)"},
	{Name: "element_count", Type: core.ConnectionTypeInteger, Label: "Element Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	presID := google.OptStr("presentation_id", inputs)
	if presID == "" {
		return google.ErrorResult("presentation_id is required")
	}
	slideID := google.OptStr("slide_id", inputs)
	if slideID == "" {
		return google.ErrorResult("slide_id is required")
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

	endpoint := fmt.Sprintf("%s/%s/pages/%s", slidesAPI, presID, slideID)

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

	var page struct {
		ObjectID     string                   `json:"objectId"`
		PageElements []map[string]interface{} `json:"pageElements"`
	}
	_ = json.Unmarshal(body, &page)

	elementsJSON, _ := json.Marshal(page.PageElements)
	elementCount := int64(len(page.PageElements))

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Slide %s — %d element(s)", slideID, elementCount),
		"slide":         string(body),
		"elements":      string(elementsJSON),
		"element_count": elementCount,
		"success":       true,
		"error":         "",
	}, nil
}
