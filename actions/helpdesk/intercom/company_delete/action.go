package helpdesk_intercom_company_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Delete Company"
	Description  = "Permanently delete a company from Intercom by its Intercom ID. This can't be undone."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+trash"
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
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Company ID (Intercom)"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	if err := intercom.DeleteResource(auth, "/companies/"+url.PathEscape(id)); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.SuccessResult(id, map[string]interface{}{"id": id, "deleted": true}, fmt.Sprintf("Deleted company %s", id)), nil
}
