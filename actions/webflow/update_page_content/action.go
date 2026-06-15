package webflow_update_page_content

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Update Page Content"
	Description  = "Update the DOM content nodes of a Webflow page"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+pencil"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Webflow API Token",
		Placeholder: "wfl_...",
		Required:    true,
	},
	{
		Name:        "page_id",
		Type:        core.ConnectionTypeSecret,
		Label:       "Page ID",
		Placeholder: "The page ID",
		Required:    true,
	},
	{
		Name:        "nodes",
		Type:        core.ConnectionTypeText,
		Label:       "Nodes",
		Placeholder: `[{"nodeId": "...", "text": "..."}]`,
		Required:    true,
	},
	{
		Name:        "locale_id",
		Type:        core.ConnectionTypeString,
		Label:       "Locale ID",
		Placeholder: "The locale identifier",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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

	nodesStr, err := webflow.RequiredString("nodes", inputs)
	if err != nil {
		return nil, err
	}

	localeID, err := webflow.RequiredString("locale_id", inputs)
	if err != nil {
		return nil, err
	}

	var nodesData interface{}
	if err := json.Unmarshal([]byte(nodesStr), &nodesData); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Invalid nodes JSON: %s", err))
	}

	reqBody := map[string]interface{}{
		"nodes":    nodesData,
		"localeId": localeID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to build request body: %s", err))
	}

	status, body, err := webflow.ExecuteRequest(token, "POST", "/pages/"+pageID+"/dom", bodyBytes)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to update page content: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	return map[string]interface{}{
		"tool_result": "Page content updated successfully",
		"success":     true,
		"error":       "",
	}, nil
}
