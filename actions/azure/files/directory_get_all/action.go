package azure_files_directory_get_all

import (
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: List Directory"
	Description  = "List a directory's contents. Unlike blob listing, which fakes hierarchy with name prefixes, this returns REAL entries — each tagged file or directory in the type field"
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
	{Name: "share", Type: core.ConnectionTypeString, Label: "Share", Placeholder: "my-share", Required: true},
	{Name: "directory", Type: core.ConnectionTypeString, Label: "Directory", Placeholder: "Leave blank for the share's root, or e.g. reports/2026"},
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "Only entries whose name starts with this"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every entry is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max entries to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Entries"},
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
	share, err := files.RequiredString("share", inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	dir := strings.Trim(files.OptionalString("directory", inputs), "/")
	if dir != "" {
		if err := files.ValidateFilePath("directory", dir); err != nil {
			return files.ErrorResult(err.Error()), nil
		}
	}

	q := url.Values{"restype": []string{"directory"}, "comp": []string{"list"}}
	if prefix := files.OptionalString("prefix", inputs); prefix != "" {
		q.Set("prefix", prefix)
	}
	returnAll := files.OptionalBool("return_all", inputs)
	limit := files.ClampLimit(files.OptionalInt("limit", inputs))

	_, dirs, fileEntries, truncated, err := files.ListEnumeration(flow, auth, files.DirectoryPath(share, dir), q, returnAll, limit)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	// Directories first, then files. The service interleaves them in one
	// <Entries> element and no cursor depends on that order, so grouping them
	// is the friendlier read.
	items := make([]interface{}, 0, len(dirs)+len(fileEntries))
	for _, d := range dirs {
		items = append(items, files.EntryMap(d, "directory"))
	}
	for _, f := range fileEntries {
		items = append(items, files.EntryMap(f, "file"))
	}

	where := "the root of share " + share
	if dir != "" {
		where = dir + " in share " + share
	}
	summary := fmt.Sprintf("Listed %d entries (%d directories, %d files) in %s", len(items), len(dirs), len(fileEntries), where)
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return files.ListResult(items, summary), nil
}
