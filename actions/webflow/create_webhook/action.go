package webflow_create_webhook

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Webhook"
	Description  = "Create a webhook for a Webflow site to receive event notifications"
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
		Name:        "site_id",
		Type:        core.ConnectionTypeString,
		Label:       "Site ID",
		Placeholder: "The Webflow site ID",
		Required:    true,
	},
	{
		Name:        "url",
		Type:        core.ConnectionTypeString,
		Label:       "Webhook URL",
		Placeholder: "https://example.com/webhook",
		Required:    true,
	},
	{
		Name:     "trigger_type",
		Type:     core.ConnectionTypeString,
		Label:    "Trigger Type",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Form Submission", Value: "form_submission"},
			{Name: "Site Publish", Value: "site_publish"},
			{Name: "Page Created", Value: "page_created"},
			{Name: "Page Metadata Updated", Value: "page_metadata_updated"},
			{Name: "Page Deleted", Value: "page_deleted"},
			{Name: "E-commerce New Order", Value: "ecomm_new_order"},
			{Name: "E-commerce Order Changed", Value: "ecomm_order_changed"},
			{Name: "E-commerce Inventory Changed", Value: "ecomm_inventory_changed"},
			{Name: "Collection Item Created", Value: "collection_item_created"},
			{Name: "Collection Item Changed", Value: "collection_item_changed"},
			{Name: "Collection Item Deleted", Value: "collection_item_deleted"},
			{Name: "Collection Item Unpublished", Value: "collection_item_unpublished"},
			{Name: "User Account Added", Value: "user_account_added"},
			{Name: "User Account Updated", Value: "user_account_updated"},
			{Name: "User Account Deleted", Value: "user_account_deleted"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
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

	webhookURL, err := webflow.RequiredString("url", inputs)
	if err != nil {
		return nil, err
	}

	triggerType, err := webflow.RequiredString("trigger_type", inputs)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]interface{}{
		"triggerType": triggerType,
		"url":         webhookURL,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to build request body: %s", err))
	}

	status, body, err := webflow.ExecuteRequest(token, "POST", "/sites/"+siteID+"/webhooks", bodyBytes)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to create webhook: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var webhook struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &webhook)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created webhook %s for %s events", webhook.ID, triggerType),
		"webhook_id":  webhook.ID,
		"success":     true,
		"error":       "",
	}, nil
}
