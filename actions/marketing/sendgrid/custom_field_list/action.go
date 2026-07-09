package marketing_sendgrid_custom_field_list

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Custom Fields"
	Description  = "List the custom field definitions in your SendGrid Marketing account. Use each field's ID (for example e1_T) as the key when setting custom field values on a contact. Optionally include SendGrid's built-in reserved fields too."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+list"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{Name: "include_reserved", Type: core.ConnectionTypeBoolean, Label: "Include Reserved Fields", Placeholder: "Tick to also return SendGrid's built-in reserved fields (first_name, email, ...) alongside your custom fields"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Field Definitions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/field_definitions", nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape"), nil
	}
	custom, _ := obj["custom_fields"].([]interface{})
	results := make([]interface{}, 0, len(custom))
	results = append(results, custom...)
	// The endpoint answers custom and reserved fields as separate arrays; when
	// the reserved ones are included they are merged in with a reserved:true
	// marker so a Loop can tell them apart.
	if v, _ := sendgrid.OptionalBoolSet("include_reserved", inputs); v {
		reserved, _ := obj["reserved_fields"].([]interface{})
		for _, r := range reserved {
			if m, ok := r.(map[string]interface{}); ok {
				m["reserved"] = true
				results = append(results, m)
			} else {
				results = append(results, r)
			}
		}
	}
	return sendgrid.ListResult(results, len(results), fmt.Sprintf("Retrieved %d field definition(s)", len(results))), nil
}
