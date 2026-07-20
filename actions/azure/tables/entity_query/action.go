package azure_tables_entity_query

import (
	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Query Rows"
	Description  = "Query rows with an OData filter. Include the Partition Key in the filter wherever you can — only Partition Key and Row Key are indexed, so any other filter scans the whole table and gets slower as it grows"
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
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter (OData)", Placeholder: `PartitionKey eq 'orders' and Total gt 100 — text values go in single quotes, and an apostrophe inside one must be doubled (O''Brien)`},
	{Name: "select", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Customer,Total — comma-separated; leave blank for every field"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow the continuation to the end instead of returning one page. On an unfiltered table this reads every row"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (per page)", Placeholder: "50 unless set — the service caps a page at 1000 rows", Visible: &core.VisibleWhen{Field: "return_all", Values: []string{"", "false"}}},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Rows"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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
	client, err := tables.TableClient(auth, table)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	// MetadataFormatMinimal, not None: on a query the per-row etag exists ONLY
	// as the body's odata.etag (there is no per-row response header), and the
	// etag is what feeds Update/Delete Row's concurrency check. ShapeEntity
	// lifts it to a plain "etag" key and drops the rest of the noise.
	opts := &aztables.ListEntitiesOptions{
		Top:    to.Ptr(tables.PageLimit(inputs)),
		Format: to.Ptr(aztables.MetadataFormatMinimal),
	}
	if filter := tables.OptionalString("filter", inputs); filter != "" {
		opts.Filter = to.Ptr(filter)
	}
	if fields := tables.OptionalString("select", inputs); fields != "" {
		opts.Select = to.Ptr(fields)
	}

	returnAll := tables.OptionalBool("return_all", inputs)
	pager := client.NewListEntitiesPager(opts)
	items, capped, err := tables.WalkPages(flow, returnAll, pager.More, func() (interface{}, error) {
		page, err := pager.NextPage(tables.Context(flow))
		if err != nil {
			return nil, err
		}
		out := make([]interface{}, 0, len(page.Entities))
		for _, raw := range page.Entities {
			entity, err := tables.ShapeEntity(raw, "")
			if err != nil {
				return nil, err
			}
			out = append(out, entity)
		}
		return out, nil
	})
	if err != nil {
		return tables.ErrorResult(auth.Errorf(err)), nil
	}
	return tables.ListResult(items, tables.ListSummary("row", len(items), returnAll, capped)), nil
}
