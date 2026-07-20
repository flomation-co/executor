package azure_tables_service_get_properties

import (
	"fmt"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Get Service Properties"
	Description  = "Read the account-level Table service settings — CORS rules, logging and metrics — and optionally the geo-replication status. Diagnostic; rarely what a flow needs"
	Website      = "https://www.flomation.co"
	Icon         = "azure+gear"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key", "entra"}}},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Connection String", Value: "connection_string"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "connection_string", Type: core.ConnectionTypeSecret, Label: "Connection String", Placeholder: "DefaultEndpointsProtocol=https;AccountName=…;AccountKey=…;EndpointSuffix=core.windows.net — Storage Account ▸ Access keys ▸ Connection string", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"connection_string"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Table Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.table.core.windows.net — leave blank to derive; Azurite: http://host:10002/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "include_geo_replication", Type: core.ConnectionTypeBoolean, Label: "Include Geo-Replication Status", Placeholder: "Only works on a read-access geo-redundant (RA-GRS) account AND only via its -secondary endpoint, e.g. https://myaccount-secondary.table.core.windows.net"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Storage Account"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Service Properties"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := tables.GetAuth(inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	svc, err := tables.ServiceClient(auth)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	props, err := svc.GetProperties(tables.Context(flow), nil)
	if err != nil {
		return tables.ErrorResult(auth.Errorf(err)), nil
	}
	result := tables.ShapeServiceProperties(props.ServiceProperties)

	// Geo-replication statistics are served ONLY by the -secondary endpoint of
	// an RA-GRS account, so asking a primary endpoint for them always fails.
	// It is opt-in rather than always-on for exactly that reason, and the
	// failure is reported rather than swallowed — a silently absent field would
	// read as "not replicated", which is a different and wrong answer.
	if tables.OptionalBool("include_geo_replication", inputs) {
		stats, err := svc.GetStatistics(tables.Context(flow), nil)
		if err != nil {
			return tables.ErrorResult(fmt.Sprintf("read the service properties, but the geo-replication status failed: %s — it is served only by the -secondary endpoint of a read-access geo-redundant account", auth.Errorf(err))), nil
		}
		result["geo_replication"] = tables.ShapeGeoReplication(stats.GeoReplication)
	}

	return tables.ResourceResult(auth.AccountName, result,
		fmt.Sprintf("Fetched the Table service properties for %s", auth.AccountName)), nil
}
