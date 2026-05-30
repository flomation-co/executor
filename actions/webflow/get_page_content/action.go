package webflow_get_page_content

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Page Content"
	Description  = "Get the DOM content nodes of a Webflow page"
	Website      = "https://www.flomation.co"
	Icon         = "globe"
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
		Name:        "page_id",
		Type:        core.ConnectionTypeString,
		Label:       "Page ID",
		Placeholder: "The page ID",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nodes", Type: core.ConnectionTypeObject, Label: "Nodes"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	pageID, err := webflow.RequiredString("page_id", inputs)
	if err != nil {
		return nil, err
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", "/pages/"+pageID+"/dom", nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to get page content: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	// Count top-level nodes if the response contains a nodes array
	var resp struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	_ = json.Unmarshal(body, &resp)
	nodeCount := len(resp.Nodes)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved page content with %d node(s)", nodeCount),
		"nodes":       parsed,
		"success":     true,
		"error":       "",
	}, nil
}
