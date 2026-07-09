package credit_note_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Credit Note: Create"
	Description  = "Create a Xero credit note for a contact. Returns the credit note ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "ACCRECCREDIT", Options: []core.ConnectionOption{
		{Name: "Accounts Receivable Credit (ACCRECCREDIT)", Value: "ACCRECCREDIT"},
		{Name: "Accounts Payable Credit (ACCPAYCREDIT)", Value: "ACCPAYCREDIT"},
	}},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Xero ContactID", Required: true},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-05"},
	{Name: "reference", Type: core.ConnectionTypeString, Label: "Reference"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "DRAFT"},
	{Name: "line_amount_types", Type: core.ConnectionTypeString, Label: "Line Amount Types", Placeholder: "Exclusive"},
	{Name: "currency_code", Type: core.ConnectionTypeString, Label: "Currency Code", Placeholder: "GBP"},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Description":"Credit","Quantity":1,"UnitAmount":50.00,"AccountCode":"200"}]`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"SentToContact":true}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	if t := xero_common.OptionalString("type", inputs); t != "" {
		body["Type"] = t
	} else {
		body["Type"] = "ACCRECCREDIT"
	}
	contactID, err := xero_common.RequiredString("contact_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	body["Contact"] = map[string]interface{}{"ContactID": contactID}
	xero_common.SetString(body, "Date", "date", inputs)
	xero_common.SetString(body, "Reference", "reference", inputs)
	xero_common.SetString(body, "Status", "status", inputs)
	xero_common.SetString(body, "LineAmountTypes", "line_amount_types", inputs)
	xero_common.SetString(body, "CurrencyCode", "currency_code", inputs)

	lines, err := xero_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	if lines != nil {
		body["LineItems"] = lines
	}

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/CreditNotes", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "CreditNotes")
	id, _ := obj["CreditNoteID"].(string)
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Created Xero credit note %s", id)), nil
}
