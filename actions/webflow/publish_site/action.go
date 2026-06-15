package webflow_publish_site

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
	Name         = "Publish Site"
	Description  = "Publish a Webflow site to its subdomain or specified custom domains"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+play"
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
	{
		Name:        "domain_ids",
		Type:        core.ConnectionTypeString,
		Label:       "Domain IDs",
		Placeholder: "Comma-separated domain IDs (optional)",
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

	siteID, err := webflow.RequiredString("site_id", inputs)
	if err != nil {
		return nil, err
	}

	domainIDs := webflow.OptionalString("domain_ids", inputs)

	var reqBody map[string]interface{}
	if domainIDs != "" {
		var ids []string
		for _, id := range strings.Split(domainIDs, ",") {
			trimmed := strings.TrimSpace(id)
			if trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
		reqBody = map[string]interface{}{
			"customDomains": ids,
		}
	} else {
		reqBody = map[string]interface{}{
			"publishToWebflowSubdomain": true,
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to build request body: %s", err))
	}

	status, body, err := webflow.ExecuteRequest(token, "POST", "/sites/"+siteID+"/publish", bodyBytes)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to publish site: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	return map[string]interface{}{
		"tool_result": "Site published successfully",
		"success":     true,
		"error":       "",
	}, nil
}
