package azure_files_directory_create

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
	Name         = "Azure Files: Create Directory"
	Description  = "Create a directory in a share. Azure has no mkdir -p: every parent must already exist, so nested paths need one call per level — turn on Create Parents to make those calls for you"
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
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
	{Name: "directory", Type: core.ConnectionTypeString, Label: "Directory", Placeholder: "reports/2026/q1", Required: true},
	{Name: "create_parents", Type: core.ConnectionTypeBoolean, Label: "Create Parents", Placeholder: "On by default: create every missing level of the path. Turn off to fail when the parent is missing", Value: true},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"team":"ops"} — applied to the leaf directory only`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Directory"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Directory"},
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
	dir, err := files.RequiredString("directory", inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	dir = strings.Trim(dir, "/")
	if err := files.ValidateFilePath("directory", dir); err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	metaHeaders := map[string]string{}
	if err := files.MetadataHeaders(metaHeaders, inputs, "metadata"); err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	// The parent levels are created bare — metadata belongs to the directory
	// the operator named, not to the scaffolding underneath it.
	segments := strings.Split(dir, "/")
	start := len(segments) - 1
	if files.BoolDefaultTrue("create_parents", inputs) {
		start = 0
	}
	created := 0
	for i := start; i < len(segments); i++ {
		level := strings.Join(segments[:i+1], "/")
		headers := map[string]string{}
		if i == len(segments)-1 {
			headers = metaHeaders
		}
		resp, err := files.Do(flow, auth, files.Request{
			Method:  http.MethodPut,
			Path:    files.DirectoryPath(share, level),
			Query:   url.Values{"restype": []string{"directory"}},
			Headers: headers,
		})
		if err != nil {
			return files.ErrorResult(err.Error()), nil
		}
		if err := files.CheckResponse(resp); err != nil {
			// An existing PARENT is exactly what Create Parents expects to
			// find; an existing LEAF is what the operator asked to create, so
			// that one is still an error.
			if files.ErrorCode(resp) == "ResourceAlreadyExists" && i < len(segments)-1 {
				continue
			}
			if files.ErrorCode(resp) == "ResourceAlreadyExists" {
				return files.ErrorResult(fmt.Sprintf("directory %q already exists in share %q", dir, share)), nil
			}
			if files.ErrorCode(resp) == "ParentNotFound" {
				return files.ErrorResult(fmt.Sprintf("the parent of %q does not exist — turn on Create Parents to create the whole path", level)), nil
			}
			return files.ErrorResult(err.Error()), nil
		}
		created++
	}

	result := files.HeadersResult(dir, nil)
	result["path"] = dir
	result["levelsCreated"] = created
	return files.ResourceResult(dir, result, fmt.Sprintf("Created directory %s in share %s", dir, share)), nil
}
