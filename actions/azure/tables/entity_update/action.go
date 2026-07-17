package azure_tables_entity_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Update Row"
	Description  = "Update a row that must already exist. Merge only changes the fields you supply; Replace DELETES every field you leave out. Supply an ETag to fail if the row changed since you read it"
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
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
	{Name: "entity", Type: core.ConnectionTypeObject, Label: "Row (JSON)", Placeholder: `{"PartitionKey":"orders","RowKey":"1001","Customer":"Acme","Total":42} — PartitionKey and RowKey are required and must be text. Rows are FLAT: no nested objects or arrays`, Required: true},
	{
		Name:  "update_mode",
		Type:  core.ConnectionTypeString,
		Label: "Update Mode",
		Options: []core.ConnectionOption{
			{Name: "Merge — only change the fields you supply", Value: "merge"},
			{Name: "Replace — delete any field you do not supply", Value: "replace"},
		},
	},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag", Placeholder: "Optional — take it from a previous action's result.etag to fail if the row changed since. LEAVE BLANK to overwrite whatever is there now"},
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
	table, err := tables.RequiredString("table", inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	// ParseEntity before the SDK sees anything: UpdateEntity asserts
	// entity["PartitionKey"].(string) with no comma-ok and PANICS on an entity
	// that lacks the key or carries a numeric one.
	raw, entity, err := tables.ParseEntity(inputs, "entity")
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	partitionKey, rowKey, err := tables.EntityKeys(entity)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	mode, err := tables.UpdateModeFor(inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	client, err := tables.TableClient(auth, table)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	// A nil IfMatch is the SDK's "*", i.e. overwrite unconditionally. That is
	// what a blank etag field must keep meaning — the alternative is failing
	// every update by an operator who never read the row first.
	opts := &aztables.UpdateEntityOptions{UpdateMode: mode, IfMatch: tables.ETagOption(inputs, "etag")}
	resp, err := client.UpdateEntity(tables.Context(flow), raw, opts)
	if err != nil {
		return tables.ErrorResult(auth.Errorf(err)), nil
	}

	result, err := tables.ShapeEntity(raw, string(resp.ETag))
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	id := tables.EntityID(partitionKey, rowKey)
	return tables.ResourceResult(id, result,
		fmt.Sprintf("Updated row %s in %s (%s)", id, table, tables.UpdateModeSummary(mode))), nil
}
