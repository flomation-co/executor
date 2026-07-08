package helpdesk_intercom_company_contact_detach

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Detach Contact from Company"
	Description  = "Detach a contact from a company they're currently linked to. The contact and the company both stay in Intercom — only the link between them is removed."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+link-slash"
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
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The contact to detach, e.g. 634c1287cd7e30…", Required: true},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company", Placeholder: "The company to detach them from — Intercom's company ID, not your own", Required: true},
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
	contactID, err := intercom.RequiredString("contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult("provide the ID of the contact to detach"), nil
	}
	companyID, err := intercom.RequiredString("company_id", inputs)
	if err != nil {
		return intercom.ErrorResult("pick the company to detach the contact from"), nil
	}

	path := "/contacts/" + url.PathEscape(contactID) + "/companies/" + url.PathEscape(companyID)
	if err := intercom.DeleteResource(auth, path); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	result := map[string]interface{}{"contact_id": contactID, "company_id": companyID, "detached": true}
	return intercom.SuccessResult(companyID, result, fmt.Sprintf("Detached contact %s from company %s", contactID, companyID)), nil
}
