package helpdesk_intercom_company_contact_attach

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Attach Contact to Company"
	Description  = "Attach a contact to a company so Intercom links the person to that business. Attaching the first contact is also what makes a new company visible in Intercom."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+link"
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
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The contact to attach, e.g. 634c1287cd7e30…", Required: true},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company", Placeholder: "The company to attach them to — Intercom's company ID, not your own", Required: true},
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
	contactID, err := intercom.RequiredString("contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult("provide the ID of the contact to attach"), nil
	}
	companyID, err := intercom.RequiredString("company_id", inputs)
	if err != nil {
		return intercom.ErrorResult("pick the company to attach the contact to"), nil
	}

	// The id here is the INTERCOM company id (the dropdown supplies it) — your
	// own company_id is not accepted on this endpoint.
	body := map[string]interface{}{"id": companyID}
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/contacts/"+url.PathEscape(contactID)+"/companies", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Attached contact %s to company %s", contactID, companyID)), nil
}
