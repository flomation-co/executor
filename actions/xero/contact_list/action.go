package contact_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: List"
	Description  = "List Xero contacts with optional filter, search and pagination. Returns matching contacts."
	Website      = "https://www.flomation.co"
	Icon         = "xero+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "where", Type: core.ConnectionTypeString, Label: "Filter (where)", Placeholder: `Name.Contains("Ltd")`},
	{Name: "search_term", Type: core.ConnectionTypeString, Label: "Search Term", Placeholder: "Ada"},
	{Name: "page", Type: core.ConnectionTypeString, Label: "Page", Placeholder: "1"},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if v := xero_common.OptionalString("where", inputs); v != "" {
		q.Set("where", v)
	}
	if v := xero_common.OptionalString("search_term", inputs); v != "" {
		q.Set("searchTerm", v)
	}
	if v := xero_common.OptionalString("page", inputs); v != "" {
		q.Set("page", v)
	}

	path := "/Contacts"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}

	resp, err := xero_common.DoJSON(flow, "GET", path, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	items := xero_common.Elements(resp, "Contacts")
	return xero_common.ListResult(items, fmt.Sprintf("Found %d Xero contact(s)", len(items))), nil
}
