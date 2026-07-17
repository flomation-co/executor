package azure_files_share_delete

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Delete Share"
	Description  = "Delete a file share and EVERYTHING in it — every directory and file, irrecoverably unless the account has share soft-delete enabled. Unlike a directory, a share does not have to be empty first"
	Website      = "https://www.flomation.co"
	Icon         = "azure+trash"
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
	{
		Name:        "delete_snapshots",
		Type:        core.ConnectionTypeString,
		Label:       "Snapshots",
		Placeholder: "A share with snapshots cannot be deleted unless they go too",
		Options: []core.ConnectionOption{
			{Name: "Delete the share and its snapshots", Value: "include"},
			{Name: "Delete the share, its snapshots, and break any snapshot leases", Value: "include-leased"},
			{Name: "Fail if the share has snapshots", Value: "none"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Share"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	// Default to include: a share with snapshots is refused outright without
	// the header, and "delete the share" almost never means "unless somebody
	// snapshotted it, in which case do nothing". "none" is the explicit opt-out
	// and sends no header, restoring the service's own refusal.
	headers := map[string]string{}
	switch snapshots := files.OptionalString("delete_snapshots", inputs); snapshots {
	case "", "include":
		headers["x-ms-delete-snapshots"] = "include"
	case "include-leased":
		headers["x-ms-delete-snapshots"] = "include-leased"
	case "none":
	default:
		return files.ErrorResult(fmt.Sprintf("delete_snapshots %q is not valid (use include, include-leased or none)", snapshots)), nil
	}

	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodDelete,
		Path:    files.SharePath(share),
		Query:   url.Values{"restype": []string{"share"}},
		Headers: headers,
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	return files.ResourceResult(share, map[string]interface{}{"name": share, "deleted": true}, fmt.Sprintf("Deleted share %s", share)), nil
}
