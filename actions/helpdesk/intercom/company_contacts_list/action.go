package helpdesk_intercom_company_contacts_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: List Company Contacts"
	Description  = "List the contacts (users and leads) attached to a company. Enable Return All to auto-paginate every contact. A contact attached moments ago can take a few minutes to show up here."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+user-group"
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
	{Name: "id", Type: core.ConnectionTypeString, Label: "Company ID (Intercom)", Placeholder: "The company's Intercom ID, e.g. 634c1287cd7e30… — not your own Company ID", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every attached contact (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contacts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := intercom.RequiredString("id", inputs)
	if err != nil {
		return intercom.ErrorResult("provide the company's Intercom ID (not your own Company ID)"), nil
	}
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	limit, _ := intercom.OptionalInt("limit", inputs)

	items, err := intercom.ListAll(auth, "/companies/"+url.PathEscape(id)+"/contacts", nil, "data", limit, returnAll)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ListResult(items, fmt.Sprintf("Retrieved %d contact(s) attached to company %s", len(items), id)), nil
}
