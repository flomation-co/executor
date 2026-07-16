package azure_storage_container_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: List Containers"
	Description  = "List the containers in the storage account, optionally filtered by a name prefix"
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Blob Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.blob.core.windows.net — leave blank to derive; Azurite: http://host:10000/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "Only containers whose name starts with this"},
	{Name: "include_metadata", Type: core.ConnectionTypeBoolean, Label: "Include Metadata", Placeholder: "Return each container's metadata alongside its properties"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every container is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max containers to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Containers"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := storage.GetAuth(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	q := url.Values{"comp": []string{"list"}}
	if prefix := storage.OptionalString("prefix", inputs); prefix != "" {
		q.Set("prefix", prefix)
	}
	if storage.OptionalBool("include_metadata", inputs) {
		q.Set("include", "metadata")
	}
	returnAll := storage.OptionalBool("return_all", inputs)
	limit := storage.ClampLimit(storage.OptionalInt("limit", inputs))

	containers, _, truncated, err := storage.ListEnumeration(flow, auth, "/", q, returnAll, limit)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	items := make([]interface{}, 0, len(containers))
	for _, c := range containers {
		items = append(items, storage.ContainerMap(c))
	}
	summary := fmt.Sprintf("Listed %d containers", len(items))
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return storage.ListResult(items, summary), nil
}
