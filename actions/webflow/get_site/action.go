package webflow_get_site

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Site"
	Description  = "Get details of a specific Webflow site by ID"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+eye"
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
		Name:        "site_id",
		Type:        core.ConnectionTypeSecret,
		Label:       "Site ID",
		Placeholder: "The Webflow site ID",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "site", Type: core.ConnectionTypeObject, Label: "Site"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	siteID, err := webflow.RequiredString("site_id", inputs)
	if err != nil {
		return nil, err
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", "/sites/"+siteID, nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to get site: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var site struct {
		DisplayName string `json:"displayName"`
		ShortName   string `json:"shortName"`
	}
	if err := json.Unmarshal(body, &site); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	var parsed interface{}
	_ = json.Unmarshal(body, &parsed)

	url := fmt.Sprintf("https://%s.webflow.io", site.ShortName)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Site '%s' (%s)", site.DisplayName, url),
		"site":        parsed,
		"name":        site.DisplayName,
		"url":         url,
		"success":     true,
		"error":       "",
	}, nil
}
