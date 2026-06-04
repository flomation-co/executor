package webflow_get_collection

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
	Name         = "Get Collection"
	Description  = "Get a Webflow CMS collection with its field schema"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+eye"
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
		Name:        "collection_id",
		Type:        core.ConnectionTypeString,
		Label:       "Collection ID",
		Placeholder: "The collection ID",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "collection", Type: core.ConnectionTypeObject, Label: "Collection"},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	collectionID, err := webflow.RequiredString("collection_id", inputs)
	if err != nil {
		return nil, err
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", "/collections/"+collectionID, nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to get collection: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var coll struct {
		DisplayName string `json:"displayName"`
		Fields      []struct {
			Slug string `json:"slug"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &coll); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	var parsed interface{}
	_ = json.Unmarshal(body, &parsed)

	var fieldsRaw json.RawMessage
	// Extract fields array from the full response
	var fullResp map[string]json.RawMessage
	if err := json.Unmarshal(body, &fullResp); err == nil {
		fieldsRaw = fullResp["fields"]
	}
	var fieldsObj interface{}
	if fieldsRaw != nil {
		_ = json.Unmarshal(fieldsRaw, &fieldsObj)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Collection '%s' with %d field(s):\n", coll.DisplayName, len(coll.Fields))
	for _, f := range coll.Fields {
		fmt.Fprintf(&sb, "- %s (%s)\n", f.Slug, f.Type)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"collection":  parsed,
		"fields":      fieldsObj,
		"success":     true,
		"error":       "",
	}, nil
}
