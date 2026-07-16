package azure_storage_blob_find_by_tags

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Find Blobs by Tags"
	Description  = "Search the whole storage account for blobs whose index tags match an expression, e.g. \"project\"='alpha' AND \"status\"='final'"
	Website      = "https://www.flomation.co"
	Icon         = "azure+magnifying-glass"
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
	{Name: "where", Type: core.ConnectionTypeString, Label: "Where", Placeholder: `"project"='alpha' — tag names in double quotes, values in single quotes`, Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every match is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max matches to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Matching Blobs"},
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
	where, err := storage.RequiredString("where", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	q := url.Values{"comp": []string{"blobs"}, "where": []string{where}}
	returnAll := storage.OptionalBool("return_all", inputs)
	limit := storage.ClampLimit(storage.OptionalInt("limit", inputs))

	// Account-wide search: the results carry each blob's container name and
	// the matched tags, not full properties.
	_, blobs, truncated, err := storage.ListEnumeration(flow, auth, "/", q, returnAll, limit)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	items := make([]interface{}, 0, len(blobs))
	for _, b := range blobs {
		items = append(items, storage.BlobMap(b))
	}
	summary := fmt.Sprintf("Found %d blobs matching the tag expression", len(items))
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return storage.ListResult(items, summary), nil
}
