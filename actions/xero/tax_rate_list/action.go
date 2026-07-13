package tax_rate_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Tax Rate: List"
	Description  = "List Xero tax rates, optionally filtered by a where clause. Returns matching tax rates."
	Website      = "https://www.flomation.co"
	Icon         = "xero+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "where", Type: core.ConnectionTypeString, Label: "Where Filter", Placeholder: `Status=="ACTIVE"`},
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
	if where := xero_common.OptionalString("where", inputs); where != "" {
		q.Set("where", where)
	}
	path := "/TaxRates"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := xero_common.DoJSON(flow, "GET", path, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	items := xero_common.Elements(resp, "TaxRates")
	return xero_common.ListResult(items, fmt.Sprintf("Found %d Xero tax rate(s)", len(items))), nil
}
