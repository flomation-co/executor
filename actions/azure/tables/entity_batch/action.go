package azure_tables_entity_batch

import (
	"fmt"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Batch Rows"
	Description  = "Apply up to 100 row changes as one all-or-nothing transaction. Every change must be in the SAME partition — that is the service's rule, not ours, and it is what makes the batch atomic"
	Website      = "https://www.flomation.co"
	Icon         = "azure+layer-group"
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
	{
		Name:        "actions",
		Type:        core.ConnectionTypeObject,
		Label:       "Changes (JSON array)",
		Placeholder: `[{"action":"upsert_merge","row":{"PartitionKey":"orders","RowKey":"1001","Total":42}},{"action":"delete","row":{"PartitionKey":"orders","RowKey":"1002"}}] — every row must share one PartitionKey`,
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Partition Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Batch Summary"},
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
	batch, err := tables.ParseBatch(inputs, "actions")
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	client, err := tables.TableClient(auth, table)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	if _, err := client.SubmitTransaction(tables.Context(flow), batch.Actions, nil); err != nil {
		return tables.ErrorResult(auth.BatchErrorf(err, batch)), nil
	}

	result := map[string]interface{}{
		"partition_key": batch.PartitionKey,
		"count":         len(batch.Actions),
		"row_keys":      batch.RowKeys,
	}
	return tables.ResourceResult(batch.PartitionKey, result,
		fmt.Sprintf("Applied %d change(s) to partition %s of %s as one transaction", len(batch.Actions), batch.PartitionKey, table)), nil
}
