package helpdesk_intercom_company_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Company"
	Description  = "Look up a single company by its Intercom ID, by your own Company ID, or by its exact name. Recently created or changed companies can take a few minutes to appear in Name lookups — look up by ID for instant results."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+magnifying-glass"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{
		Name:  "select_by",
		Type:  core.ConnectionTypeString,
		Label: "Find By",
		Options: []core.ConnectionOption{
			{Name: "Intercom ID", Value: "id"},
			{Name: "Company ID (yours)", Value: "company_id"},
			{Name: "Name", Value: "name"},
		},
	},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "The ID or name to look up — must match your Find By choice", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Company ID (Intercom)"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Company"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	value, err := intercom.RequiredString("value", inputs)
	if err != nil {
		return intercom.ErrorResult("provide the ID or name to look the company up by"), nil
	}
	selectBy := intercom.OptionalString("select_by", inputs)
	if selectBy == "" {
		selectBy = "id"
	}

	var raw map[string]interface{}
	switch selectBy {
	case "id":
		raw, err = intercom.GetObject(auth, "/companies/"+url.PathEscape(value), nil)
	case "company_id":
		raw, err = intercom.GetObject(auth, "/companies", url.Values{"company_id": {value}})
	case "name":
		raw, err = intercom.GetObject(auth, "/companies", url.Values{"name": {value}})
	default:
		return intercom.ErrorResult("Find By must be Intercom ID, Company ID (yours), or Name"), nil
	}
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, found := unwrapCompany(raw)
	if !found {
		return intercom.ErrorResult(fmt.Sprintf("no company found matching %q", value)), nil
	}
	label := intercom.StringifyID(obj["name"])
	if label == "" {
		label = intercom.StringifyID(obj["id"])
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Found company %s", label)), nil
}

// unwrapCompany handles the two shapes the lookup can reply with: the path
// lookup returns the company object itself, while the company_id/name filter
// lookups may return a classic list envelope — take its first match.
func unwrapCompany(raw map[string]interface{}) (map[string]interface{}, bool) {
	for _, key := range []string{"data", "companies"} {
		arr, ok := raw[key].([]interface{})
		if !ok {
			continue
		}
		if len(arr) == 0 {
			return nil, false
		}
		obj, ok := arr[0].(map[string]interface{})
		return obj, ok
	}
	if _, ok := raw["id"]; ok {
		return raw, true
	}
	return nil, false
}
