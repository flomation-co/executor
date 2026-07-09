package vendor_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Vendor: Create"
	Description  = "Create a QuickBooks Online vendor. Returns the vendor ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Acme Supplies Ltd", Required: true},
	{Name: "company_name", Type: core.ConnectionTypeString, Label: "Company Name"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "accounts@acme.com"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"BillAddr":{"Line1":"...","City":"..."}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	quickbooks_common.SetString(body, "DisplayName", "display_name", inputs)
	quickbooks_common.SetString(body, "CompanyName", "company_name", inputs)
	if v := quickbooks_common.OptionalString("email", inputs); v != "" {
		body["PrimaryEmailAddr"] = map[string]interface{}{"Address": v}
	}
	if v := quickbooks_common.OptionalString("phone", inputs); v != "" {
		body["PrimaryPhone"] = map[string]interface{}{"FreeFormNumber": v}
	}

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "vendor", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Vendor")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, fmt.Sprintf("Created QuickBooks vendor %q", quickbooks_common.OptionalString("display_name", inputs))), nil
}
