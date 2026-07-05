package purchase_order_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Purchase Order: List"
	Description  = "List Xero purchase orders, optionally filtered by status. Returns matching purchase orders."
	Website      = "https://www.flomation.co"
	Icon         = "xero+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "AUTHORISED"},
	{Name: "page", Type: core.ConnectionTypeString, Label: "Page", Placeholder: "1"},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	path := "/PurchaseOrders"
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

	items := xero_common.Elements(resp, "PurchaseOrders")
	return xero_common.ListResult(items, fmt.Sprintf("Found %d Xero purchase order(s)", len(items))), nil
}
