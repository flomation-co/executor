package azure_tables_entity_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Delete Row"
	Description  = "Delete one row by Partition Key and Row Key. Supply an ETag to fail if the row changed since you read it"
	Website      = "https://www.flomation.co"
	Icon         = "azure+trash"
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
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag", Placeholder: "Optional — take it from a previous action's result.etag to fail if the row changed since. LEAVE BLANK to overwrite whatever is there now"},
	{Name: "ignore_if_missing", Type: core.ConnectionTypeBoolean, Label: "Ignore If It Does Not Exist", Placeholder: "Treat a missing row as success instead of an error"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Partition Key / Row Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Row"},
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

	id := tables.EntityID(partitionKey, rowKey)
	echo := map[string]interface{}{"PartitionKey": partitionKey, "RowKey": rowKey}

	// A nil IfMatch is the SDK's "*" — delete whatever is there now.
	opts := &aztables.DeleteEntityOptions{IfMatch: tables.ETagOption(inputs, "etag")}
	if _, err := client.DeleteEntity(tables.Context(flow), partitionKey, rowKey, opts); err != nil {
		if tables.IsNotFound(err) && tables.OptionalBool("ignore_if_missing", inputs) {
			echo["deleted"] = false
			return tables.ResourceResult(id, echo, fmt.Sprintf("Row %s does not exist in %s", id, table)), nil
		}
		return tables.ErrorResult(auth.Errorf(err)), nil
	}
	echo["deleted"] = true
	return tables.ResourceResult(id, echo, fmt.Sprintf("Deleted row %s from %s", id, table)), nil
}
