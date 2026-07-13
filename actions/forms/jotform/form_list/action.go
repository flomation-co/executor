// Package form_list lists the JotForm forms in the account.
package form_list

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	forms_common "flomation.app/automate/executor/actions/forms"
	"flomation.app/automate/executor/actions/forms/jotform"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Forms"
	Description  = "List JotForm forms in the account, with optional limit, offset and filter."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list+list"
	Date         = "11/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "JotForm API Key", Placeholder: "${secrets.jotform_api_key}", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "us", Options: []core.ConnectionOption{
		{Name: "US (default)", Value: "us"},
		{Name: "EU", Value: "eu"},
		{Name: "HIPAA", Value: "hipaa"},
	}},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "20"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "0"},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter (JSON)", Placeholder: `{"status":"ENABLED"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Forms"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := jotform.Get(inputs)
	if err != nil {
		return forms_common.ErrorResult(err.Error()), nil
	}
	region := jotform.Region(inputs)

	query := url.Values{}
	for _, name := range []string{"limit", "offset", "filter"} {
		if v := forms_common.OptionalString(name, inputs); v != "" {
			query.Set(name, v)
		}
	}
	path := "/user/forms"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	raw, status, err := jotform.Do(jotform.Context(flow), http.MethodGet, path, apiKey, region, nil)
	if err != nil {
		return forms_common.ErrorResult(fmt.Sprintf("JotForm request failed: %v", err)), nil
	}
	if status != http.StatusOK {
		return forms_common.ErrorResult(jotform.StatusMessage(status, raw)), nil
	}

	items := jotform.ContentList(raw)
	return forms_common.ListResult(items, fmt.Sprintf("Found %d JotForm form(s).", len(items))), nil
}
