package account_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: List"
	Description  = "List Xero chart-of-accounts accounts, optionally filtered. Returns matching accounts."
	Website      = "https://www.flomation.co"
	Icon         = "xero+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "where", Type: core.ConnectionTypeString, Label: "Where Filter", Placeholder: `Type=="BANK"`},
	{Name: "order", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "Name"},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	path := "/Accounts"
	q := url.Values{}
	if v := xero_common.OptionalString("where", inputs); v != "" {
		q.Set("where", v)
	}
	if v := xero_common.OptionalString("order", inputs); v != "" {
		q.Set("order", v)
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := xero_common.DoJSON(flow, "GET", path, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	items := xero_common.Elements(resp, "Accounts")
	return xero_common.ListResult(items, fmt.Sprintf("Found %d Xero account(s)", len(items))), nil
}
