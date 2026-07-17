package azure_tables_entity_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Get Row"
	Description  = "Fetch one row by Partition Key and Row Key. This is the fast path — the only lookup that goes straight to the row rather than scanning"
	Website      = "https://www.flomation.co"
	Icon         = "azure+magnifying-glass"
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
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "MyTable — letters and digits only, starting with a letter", Required: true},
	{Name: "partition_key", Type: core.ConnectionTypeString, Label: "Partition Key", Placeholder: "The group the row lives in, e.g. orders — rows sharing one Partition Key are stored together and are the only rows a batch can touch", Required: true},
	{Name: "row_key", Type: core.ConnectionTypeString, Label: "Row Key", Placeholder: "The row's ID within that group, e.g. 1001 — Partition Key + Row Key together identify exactly one row", Required: true},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Customer,Total — comma-separated; leave blank for every field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Partition Key / Row Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Row"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := tables.GetAuth(inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	table, partitionKey, rowKey, err := tables.PointArgs(inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	client, err := tables.TableClient(auth, table)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	// MetadataFormatMinimal, matching entity_query — NOT None.
	//
	// The metadata level is what decides whether the service sends the
	// "Prop@odata.type" sidecars, and those sidecars ARE the type system: an
	// Edm.Int64 travels as a plain JSON STRING, an Edm.DateTime/Binary/Guid the
	// same, and the sidecar is the only thing distinguishing them from an
	// Edm.String. Under odata=nometadata the service sends none of them, so a
	// row read here and written back by entity_upsert — the ordinary Get → edit
	// → Upsert flow — silently RETYPED every one of those properties to
	// Edm.String, losing Int64 precision on the next read. Verified live against
	// both Azurite and a real account.
	//
	// None was chosen to keep odata.metadata (which echoes the endpoint URL)
	// out of the flow output, but that was never what protected it: ShapeEntity
	// drops odata.metadata unconditionally, on this path and on entity_query's,
	// which has always run at Minimal for this exact reason.
	opts := &aztables.GetEntityOptions{Format: to.Ptr(aztables.MetadataFormatMinimal)}
	resp, err := client.GetEntity(tables.Context(flow), partitionKey, rowKey, opts)
	if err != nil {
		return tables.ErrorResult(auth.Errorf(err)), nil
	}

	result, err := tables.ShapeEntity(resp.Value, string(resp.ETag))
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	if fields := tables.OptionalString("select", inputs); fields != "" {
		// The Table service applies $select only on query, not on a point read,
		// so the projection happens here rather than being silently ignored.
		result = tables.SelectFields(result, fields)
	}
	return tables.ResourceResult(tables.EntityID(partitionKey, rowKey), result,
		fmt.Sprintf("Fetched row %s from %s", tables.EntityID(partitionKey, rowKey), table)), nil
}
