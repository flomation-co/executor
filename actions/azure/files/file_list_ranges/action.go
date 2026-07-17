package azure_files_file_list_ranges

import (
	"encoding/xml"
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
	Name         = "Azure Files: List File Ranges"
	Description  = "List the byte ranges of a file that have actually been written. Azure Files allocates sparsely, so a file's size and its written content are different questions — this answers the second one"
	Website      = "https://www.flomation.co"
	Icon         = "azure+layer-group"
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
	{Name: "range", Type: core.ConnectionTypeString, Label: "Byte Range", Placeholder: "bytes=0-1023 — leave blank to list the whole file"},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the file is leased — the Lease ID output of a Lease File step"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Ranges"},
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
	fileName, err := files.RequiredString("file_name", inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	logical := files.JoinPath(dir, fileName)

	headers := map[string]string{}
	if r := files.OptionalString("range", inputs); r != "" {
		if !strings.HasPrefix(r, "bytes=") {
			return files.ErrorResult(`range must look like "bytes=0-1023"`), nil
		}
		headers["x-ms-range"] = r
	}
	headers = files.LeaseHeader(headers, inputs)

	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodGet,
		Path:    files.FilePath(share, dir, fileName),
		Query:   url.Values{"comp": []string{"rangelist"}},
		Headers: headers,
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	var list files.RangeList
	if err := xml.Unmarshal(resp.Body, &list); err != nil {
		return files.ErrorResult(fmt.Sprintf("failed to parse the range list response: %s", err.Error())), nil
	}

	// Written and cleared ranges are tagged rather than split into two outputs:
	// they describe the same axis, and a flow reading them wants one ordered
	// picture of the file.
	var written int64
	items := make([]interface{}, 0, len(list.Ranges)+len(list.ClearRanges))
	for _, r := range list.Ranges {
		written += r.End - r.Start + 1
		items = append(items, map[string]interface{}{"start": r.Start, "end": r.End, "bytes": r.End - r.Start + 1, "type": "range"})
	}
	for _, r := range list.ClearRanges {
		items = append(items, map[string]interface{}{"start": r.Start, "end": r.End, "bytes": r.End - r.Start + 1, "type": "clear"})
	}
	return files.ListResult(items, fmt.Sprintf("%s has %d written ranges covering %d bytes", logical, len(list.Ranges), written)), nil
}
