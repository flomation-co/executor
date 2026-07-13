package quote_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Quote: List"
	Description  = "List Xero quotes, optionally filtered by status. Returns matching quotes."
	Website      = "https://www.flomation.co"
	Icon         = "xero+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "ACCEPTED"},
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

	path := "/Quotes"
	q := url.Values{}
	if v := xero_common.OptionalString("status", inputs); v != "" {
		q.Set("Status", v)
	}
	if v := xero_common.OptionalString("page", inputs); v != "" {
		q.Set("page", v)
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := xero_common.DoJSON(flow, "GET", path, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	items := xero_common.Elements(resp, "Quotes")
	return xero_common.ListResult(items, fmt.Sprintf("Found %d Xero quote(s)", len(items))), nil
}
