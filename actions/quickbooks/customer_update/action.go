package customer_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Customer: Update"
	Description  = "Update a QuickBooks Online customer (sparse). Requires ID and sync token."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "42", Required: true},
	{Name: "sync_token", Type: core.ConnectionTypeString, Label: "Sync Token", Placeholder: "3", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Ada Lovelace Ltd"},
	{Name: "company_name", Type: core.ConnectionTypeString, Label: "Company Name"},
	{Name: "given_name", Type: core.ConnectionTypeString, Label: "First Name"},
	{Name: "family_name", Type: core.ConnectionTypeString, Label: "Last Name"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Notes":"..."}`},
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

	id, err := quickbooks_common.RequiredString("id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	syncToken, err := quickbooks_common.RequiredString("sync_token", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"Id":        id,
		"SyncToken": syncToken,
		"sparse":    true,
	}
	quickbooks_common.SetString(body, "DisplayName", "display_name", inputs)
	quickbooks_common.SetString(body, "CompanyName", "company_name", inputs)
	quickbooks_common.SetString(body, "GivenName", "given_name", inputs)
	quickbooks_common.SetString(body, "FamilyName", "family_name", inputs)
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

	resp, err := quickbooks_common.Post(flow, auth, "customer", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Customer")
	return quickbooks_common.ObjectResult(quickbooks_common.IDOf(obj), obj, fmt.Sprintf("Updated QuickBooks customer %q", id)), nil
}
