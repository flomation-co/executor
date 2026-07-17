package azure_files_file_lease

import (
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Lease File"
	Description  = "Take, swap, or release a write lock on a file. Acquire returns a Lease ID — pass it to the Lease ID field of any Upload/Delete/Set step to write to the locked file; every write WITHOUT it is refused while the lease is held. A file lease is INFINITE: it outlives the flow that took it, so release it. Break ends a lease you do not hold"
	Website      = "https://www.flomation.co"
	Icon         = "azure+lock"
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
	{
		Name:     "lease_action",
		Type:     core.ConnectionTypeString,
		Label:    "Lease Action",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Acquire — take the lock", Value: "acquire"},
			{Name: "Change — swap the lock's ID", Value: "change"},
			{Name: "Release — hand the lock back", Value: "release"},
			{Name: "Break — end someone else's lock", Value: "break"},
		},
	},
	{
		Name:        "lease_id",
		Type:        core.ConnectionTypeString,
		Label:       "Lease ID",
		Placeholder: "The Lease ID output of the Acquire step — optional on Break, which does not need it",
		Visible:     &core.VisibleWhen{Field: "lease_action", Values: []string{"change", "release", "break"}},
	},
	{
		Name:        "proposed_lease_id",
		Type:        core.ConnectionTypeString,
		Label:       "Proposed Lease ID",
		Placeholder: "A GUID to use as the lease's ID — leave blank on Acquire to let Azure choose one",
		Visible:     &core.VisibleWhen{Field: "lease_action", Values: []string{"acquire", "change"}},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "File Path"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID"},
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

	call, err := files.BuildLeaseCall(inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodPut,
		Path:    files.FilePath(share, dir, fileName),
		Query:   url.Values{"comp": []string{"lease"}},
		Headers: call.Headers,
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	return files.LeaseResult(call, logical, logical, resp), nil
}
