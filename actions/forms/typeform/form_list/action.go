// Package form_list lists the Typeform forms in a workspace/account.
package form_list

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/typeform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Forms"
	Description  = "List Typeform forms in the account, with optional search and page size."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+list"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Typeform Personal Access Token", Placeholder: "${secrets.typeform_token}", Required: true},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "contact"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "10"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Forms"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_items", Type: core.ConnectionTypeInteger, Label: "Total Items"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := typeform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}

	query := url.Values{}
	if v := forms_common.OptionalString("search", inputs); v != "" {
		query.Set("search", v)
	}
	if v := forms_common.OptionalString("page_size", inputs); v != "" {
		query.Set("page_size", v)
	}
	path := "/forms"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	obj, status, err := typeform.Do(typeform.Context(flow), http.MethodGet, path, token, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("Typeform request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(typeform.StatusMessage(status, obj)), nil
	}

	items := make([]map[string]interface{}, 0)
	if rawItems, ok := obj["items"].([]interface{}); ok {
		for _, it := range rawItems {
			if m, ok := it.(map[string]interface{}); ok {
				items = append(items, m)
			}
		}
	}
	totalItems := 0
	if tc, ok := obj["total_items"].(float64); ok {
		totalItems = int(tc)
	}

	result := forms_common.ListResult(items, fmt.Sprintf("Found %d Typeform form(s) (of %d total).", len(items), totalItems))
	result["total_items"] = totalItems
	return result, nil
}
