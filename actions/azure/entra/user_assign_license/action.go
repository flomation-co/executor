package azure_entra_user_assign_license

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Entra ID: Assign License"
	Description  = "Add and/or remove licence SKUs on a user. Find skuId GUIDs with Get Subscribed SKUs. The user must have a usageLocation set first, or Graph refuses the assignment (the error says how to fix it). Requires the User.ReadWrite.All application permission."
	Website      = "https://www.flomation.co"
	Icon         = "azure+key"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Required: true},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the app registration", Required: true},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID", Required: true},
	{Name: "graph_endpoint", Type: core.ConnectionTypeString, Label: "Graph Endpoint", Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID or UPN", Placeholder: "Object ID (GUID) or jane.doe@your-tenant.onmicrosoft.com", Required: true},
	{Name: "add_sku_ids", Type: core.ConnectionTypeString, Label: "Add SKU IDs", Placeholder: "Comma-separated skuId GUIDs to assign (see Get Subscribed SKUs)"},
	{Name: "remove_sku_ids", Type: core.ConnectionTypeString, Label: "Remove SKU IDs", Placeholder: "Comma-separated skuId GUIDs to remove"},
	{Name: "disabled_plans", Type: core.ConnectionTypeString, Label: "Disabled Plans", Placeholder: "Comma-separated servicePlanId GUIDs to disable on the ADDED SKUs"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := entra.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	userID, err := entra.RequiredString("user_id", inputs)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}

	addSkus := entra.SplitCommaList(entra.OptionalString("add_sku_ids", inputs))
	removeSkus := entra.SplitCommaList(entra.OptionalString("remove_sku_ids", inputs))
	if len(addSkus) == 0 && len(removeSkus) == 0 {
		return entra.ErrorResult("provide add_sku_ids and/or remove_sku_ids — nothing to assign"), nil
	}

	// disabledPlans applies to every SKU being added; Graph takes it per
	// addLicenses entry.
	disabledPlans := []interface{}{}
	for _, p := range entra.SplitCommaList(entra.OptionalString("disabled_plans", inputs)) {
		disabledPlans = append(disabledPlans, p)
	}
	addLicenses := []interface{}{}
	for _, sku := range addSkus {
		entry := map[string]interface{}{"skuId": sku, "disabledPlans": disabledPlans}
		addLicenses = append(addLicenses, entry)
	}
	removeLicenses := []interface{}{}
	for _, sku := range removeSkus {
		removeLicenses = append(removeLicenses, sku)
	}
	body := map[string]interface{}{
		"addLicenses":    addLicenses,
		"removeLicenses": removeLicenses,
	}

	resp, err := entra.ExecuteAPI(flow, auth, "POST", "/users/"+url.PathEscape(userID)+"/assignLicense", body)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	if err := entra.CheckResponse(resp); err != nil {
		// The needs-usageLocation failure is friendly-mapped by CheckResponse.
		return entra.ErrorResult(err.Error()), nil
	}
	obj, err := entra.Decode(resp)
	if err != nil {
		return entra.ErrorResult(err.Error()), nil
	}
	out := entra.ResourceResult(obj, fmt.Sprintf("Assigned licences on user %s (+%d / -%d SKU(s))", userID, len(addSkus), len(removeSkus)))
	if out["id"] == "" {
		out["id"] = userID
	}
	return out, nil
}
