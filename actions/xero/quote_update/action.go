package quote_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Quote: Update"
	Description  = "Update an existing Xero quote by its ID. Returns the updated quote object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Customer Contact ID", Placeholder: "00000000-0000-0000-0000-000000000000"},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-05"},
	{Name: "reference", Type: core.ConnectionTypeString, Label: "Reference", Placeholder: "QU-1001"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Website build"},
	{Name: "summary", Type: core.ConnectionTypeString, Label: "Summary", Placeholder: "Quote summary line"},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Description":"Design","Quantity":1,"UnitAmount":500.0,"AccountCode":"200"}]`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Status":"SENT"}`},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	id, err := xero_common.RequiredString("quote_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"QuoteID": id}
	if contactID := xero_common.OptionalString("contact_id", inputs); contactID != "" {
		body["Contact"] = map[string]interface{}{"ContactID": contactID}
	}
	xero_common.SetString(body, "Date", "date", inputs)
	xero_common.SetString(body, "Reference", "reference", inputs)
	xero_common.SetString(body, "Title", "title", inputs)
	xero_common.SetString(body, "Summary", "summary", inputs)

	lineItems, err := xero_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	if lineItems != nil {
		body["LineItems"] = lineItems
	}

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/Quotes", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Quotes")
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Updated Xero quote %s", id)), nil
}
