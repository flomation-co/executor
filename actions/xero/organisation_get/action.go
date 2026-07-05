package organisation_get

import (
	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Organisation: Get"
	Description  = "Fetch the connected Xero organisation's details. Returns the organisation object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+book"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	resp, err := xero_common.DoJSON(flow, "GET", "/Organisations", token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Organisations")
	id, _ := obj["OrganisationID"].(string)
	name, _ := obj["Name"].(string)
	summary := "Fetched Xero organisation"
	if name != "" {
		summary = "Fetched Xero organisation " + name
	}
	return xero_common.ObjectResult(id, obj, summary), nil
}
