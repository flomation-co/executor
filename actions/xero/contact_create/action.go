package contact_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: Create"
	Description  = "Create a Xero contact (customer or supplier). Returns the contact ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Ada Lovelace Ltd", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Ada"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Lovelace"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "ada@example.com"},
	{Name: "contact_number", Type: core.ConnectionTypeString, Label: "Contact Number", Placeholder: "Your reference for this contact"},
	{Name: "account_number", Type: core.ConnectionTypeString, Label: "Account Number"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Phones":[{"PhoneType":"DEFAULT","PhoneNumber":"..."}]}`},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	xero_common.SetString(body, "Name", "name", inputs)
	xero_common.SetString(body, "FirstName", "first_name", inputs)
	xero_common.SetString(body, "LastName", "last_name", inputs)
	xero_common.SetString(body, "EmailAddress", "email", inputs)
	xero_common.SetString(body, "ContactNumber", "contact_number", inputs)
	xero_common.SetString(body, "AccountNumber", "account_number", inputs)

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/Contacts", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Contacts")
	id, _ := obj["ContactID"].(string)
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Created Xero contact %q", xero_common.OptionalString("name", inputs))), nil
}
