package webflow_get_form_submissions

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Form Submissions"
	Description  = "Get submissions for a Webflow form with pagination"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+file-lines"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeString,
		Label:       "Webflow API Token",
		Placeholder: "wfl_...",
		Required:    true,
	},
	{
		Name:        "form_id",
		Type:        core.ConnectionTypeString,
		Label:       "Form ID",
		Placeholder: "The form ID",
		Required:    true,
	},
	{
		Name:        "limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "Limit",
		Placeholder: "Number of submissions to return",
	},
	{
		Name:        "offset",
		Type:        core.ConnectionTypeInteger,
		Label:       "Offset",
		Placeholder: "Number of submissions to skip",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "submissions", Type: core.ConnectionTypeObject, Label: "Submissions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	formID, err := webflow.RequiredString("form_id", inputs)
	if err != nil {
		return nil, err
	}

	path := "/forms/" + formID + "/submissions"
	var params []string
	if limit := webflow.OptionalInt("limit", inputs); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if offset := webflow.OptionalInt("offset", inputs); offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", offset))
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", path, nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to get form submissions: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var resp struct {
		FormSubmissions []json.RawMessage `json:"formSubmissions"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	var parsed []interface{}
	for _, raw := range resp.FormSubmissions {
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d submission(s)", len(resp.FormSubmissions)),
		"submissions": parsed,
		"count":       len(resp.FormSubmissions),
		"success":     true,
		"error":       "",
	}, nil
}
