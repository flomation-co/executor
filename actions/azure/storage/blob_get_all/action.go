package azure_storage_blob_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: List Blobs"
	Description  = "List the blobs in a container, optionally filtered by a name prefix and enriched with metadata, snapshots, versions, soft-deleted blobs, or tags"
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "my-container", Required: true},
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "Only blobs whose name starts with this, e.g. reports/2026/"},
	{
		Name:        "include",
		Type:        core.ConnectionTypeComboBox,
		Label:       "Include",
		Placeholder: "Leave blank for nothing extra; combine with commas, e.g. metadata,tags",
		Options: []core.ConnectionOption{
			{Name: "Metadata", Value: "metadata"},
			{Name: "Index tags", Value: "tags"},
			{Name: "Metadata + index tags", Value: "metadata,tags"},
			{Name: "Snapshots", Value: "snapshots"},
			{Name: "Versions", Value: "versions"},
			{Name: "Soft-deleted blobs", Value: "deleted"},
			{Name: "Soft-deleted blobs with versions", Value: "deletedwithversions"},
			{Name: "Uncommitted blocks (failed/abandoned uploads)", Value: "uncommittedblobs"},
			{Name: "Copy state", Value: "copy"},
			{Name: "Permissions (hierarchical namespace)", Value: "permissions"},
			{Name: "Immutability policy", Value: "immutabilitypolicy"},
			{Name: "Legal hold", Value: "legalhold"},
		},
	},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every blob is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max blobs to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Blobs"},
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
	container, err := storage.RequiredString("container", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	q := url.Values{"restype": []string{"container"}, "comp": []string{"list"}}
	if prefix := storage.OptionalString("prefix", inputs); prefix != "" {
		q.Set("prefix", prefix)
	}
	include, err := storage.ParseIncludeTokens(storage.OptionalString("include", inputs), storage.BlobIncludeTokens)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if include != "" {
		q.Set("include", include)
	}
	returnAll := storage.OptionalBool("return_all", inputs)
	limit := storage.ClampLimit(storage.OptionalInt("limit", inputs))

	_, blobs, truncated, err := storage.ListEnumeration(flow, auth, storage.ContainerPath(container), q, returnAll, limit)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	items := make([]interface{}, 0, len(blobs))
	for _, b := range blobs {
		items = append(items, storage.BlobMap(b))
	}
	summary := fmt.Sprintf("Listed %d blobs in %s", len(items), container)
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return storage.ListResult(items, summary), nil
}
