package azure_files_share_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: List Shares"
	Description  = "List the file shares in the storage account, optionally filtered by a name prefix and enriched with metadata, snapshots, or soft-deleted shares"
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage File Data SMB/Privileged role. Azure requires backup intent on OAuth calls, which BYPASSES the share's file permissions — use Shared Key if the ACLs must apply", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.file.core.windows.net — leave blank to derive; sovereign clouds only (Azurite has no File service)"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "Only shares whose name starts with this"},
	{
		Name:        "include",
		Type:        core.ConnectionTypeComboBox,
		Label:       "Include",
		Placeholder: "Leave blank for nothing extra; combine with commas, e.g. metadata,snapshots",
		Options: []core.ConnectionOption{
			{Name: "Metadata", Value: "metadata"},
			{Name: "Snapshots", Value: "snapshots"},
			{Name: "Metadata + snapshots", Value: "metadata,snapshots"},
			{Name: "Soft-deleted shares", Value: "deleted"},
		},
	},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every share is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max shares to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Shares"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := files.GetAuth(inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	q := url.Values{"comp": []string{"list"}}
	if prefix := files.OptionalString("prefix", inputs); prefix != "" {
		q.Set("prefix", prefix)
	}
	include, err := files.ParseIncludeTokens(files.OptionalString("include", inputs), files.ShareIncludeTokens)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if include != "" {
		q.Set("include", include)
	}
	returnAll := files.OptionalBool("return_all", inputs)
	limit := files.ClampLimit(files.OptionalInt("limit", inputs))

	shares, _, _, truncated, err := files.ListEnumeration(flow, auth, "/", q, returnAll, limit)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	items := make([]interface{}, 0, len(shares))
	for _, s := range shares {
		items = append(items, files.ShareMap(s))
	}
	summary := fmt.Sprintf("Listed %d shares", len(items))
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return files.ListResult(items, summary), nil
}
