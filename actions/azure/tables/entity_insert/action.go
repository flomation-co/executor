package azure_tables_entity_insert

import (
	"fmt"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Insert Row"
	Description  = "Insert a new row. Fails if a row with the same Partition Key and Row Key already exists — this is the strictly-create path; use Upsert Row to insert-or-update"
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
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
	raw, entity, err := tables.ParseEntity(inputs, "entity")
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	partitionKey, rowKey, err := tables.EntityKeys(entity)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	client, err := tables.TableClient(auth, table)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	resp, err := client.AddEntity(tables.Context(flow), raw, nil)
	if err != nil {
		return tables.ErrorResult(auth.Errorf(err)), nil
	}

	result, err := tables.ShapeEntity(raw, string(resp.ETag))
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	return tables.ResourceResult(tables.EntityID(partitionKey, rowKey), result,
		fmt.Sprintf("Inserted row %s into %s", tables.EntityID(partitionKey, rowKey), table)), nil
}
