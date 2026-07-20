package azure_files_file_delete

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Delete File"
	Description  = "Delete a file from a share. The file's directory is left in place"
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
	{Name: "directory", Type: core.ConnectionTypeString, Label: "Directory", Placeholder: "Leave blank for the share's root, or e.g. reports/2026"},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name", Placeholder: "summary.pdf", Required: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the file is leased — the Lease ID output of a Lease File step"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "File Path"},
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
	dir := strings.Trim(files.OptionalString("directory", inputs), "/")
	fileName, err := files.RequiredString("file_name", inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	logical := files.JoinPath(dir, fileName)

	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodDelete,
		Path:    files.FilePath(share, dir, fileName),
		Query:   url.Values{},
		Headers: files.LeaseHeader(nil, inputs),
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	return files.ResourceResult(logical, map[string]interface{}{"path": logical, "deleted": true},
		fmt.Sprintf("Deleted %s from share %s", logical, share)), nil
}
