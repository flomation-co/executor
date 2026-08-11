// Package get_template fetches a HeyGen template and its variable schema. The
// returned `variables` are in the same shape the Generate from Template action
// accepts, so you can read them, edit the values, and post them back.
package get_template

import (
	"fmt"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Template"
	Description  = "Get a HeyGen template and its variable schema (the fields you fill to generate a video)."
	Website      = "https://www.flomation.co"
	Icon         = "copy+circle-info"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Template Name"},
	{Name: "aspect_ratio", Type: core.ConnectionTypeString, Label: "Aspect Ratio"},
	{Name: "variables", Type: core.ConnectionTypeObject, Label: "Variable schema (edit and pass to Generate from Template)"},
	{Name: "template", Type: core.ConnectionTypeObject, Label: "Full template (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}
	templateID := heygen.OptionalString("template_id", inputs)
	if templateID == "" {
		return heygen.ErrorResult("template_id is required"), nil
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v3/templates/"+templateID, nil)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	name := heygen.Str(data, "name")
	aspect := heygen.Str(data, "aspect_ratio")
	var variables map[string]interface{}
	if data != nil {
		variables, _ = data["variables"].(map[string]interface{})
	}

	return heygen.Result(summarise(name, variables), map[string]interface{}{
		"name":         name,
		"aspect_ratio": aspect,
		"variables":    variables,
		"template":     data,
	}), nil
}

// summarise lists the template's variable names and types so the caller (or AI)
// knows what to fill.
func summarise(name string, variables map[string]interface{}) string {
	if len(variables) == 0 {
		return fmt.Sprintf("Template '%s' has no fillable variables.", name)
	}
	names := make([]string, 0, len(variables))
	for k := range variables {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, k := range names {
		t := ""
		if v, ok := variables[k].(map[string]interface{}); ok {
			t = heygen.Str(v, "type")
		}
		if t != "" {
			parts = append(parts, fmt.Sprintf("%s (%s)", k, t))
		} else {
			parts = append(parts, k)
		}
	}
	return fmt.Sprintf("Template '%s' variables: %s", name, strings.Join(parts, ", "))
}
