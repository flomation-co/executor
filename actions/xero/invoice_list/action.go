package invoice_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: List"
	Description  = "List Xero invoices with optional filter, status and pagination. Returns matching invoices."
	Website      = "https://www.flomation.co"
	Icon         = "xero+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "where", Type: core.ConnectionTypeString, Label: "Filter (where)", Placeholder: `Status=="AUTHORISED"`},
	{Name: "statuses", Type: core.ConnectionTypeString, Label: "Statuses", Placeholder: "DRAFT,AUTHORISED"},
	{Name: "page", Type: core.ConnectionTypeString, Label: "Page", Placeholder: "1"},
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

	q := url.Values{}
	if v := xero_common.OptionalString("where", inputs); v != "" {
		q.Set("where", v)
	}
	if v := xero_common.OptionalString("statuses", inputs); v != "" {
		q.Set("Statuses", v)
	}
	if v := xero_common.OptionalString("page", inputs); v != "" {
		q.Set("page", v)
	}

	path := "/Invoices"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}

	resp, err := xero_common.DoJSON(flow, "GET", path, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	items := xero_common.Elements(resp, "Invoices")
	return xero_common.ListResult(items, fmt.Sprintf("Found %d Xero invoice(s)", len(items))), nil
}
