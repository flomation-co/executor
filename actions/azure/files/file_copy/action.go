package azure_files_file_copy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Copy File"
	Description  = "Start a server-side (async) file copy — from another file in the same account, or from any URL Azure can reach, including a blob — and optionally wait for it to complete. Handles files of any size, unlike Upload's 256 MB ceiling"
	Website      = "https://www.flomation.co"
	Icon         = "azure+copy"
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
	{Name: "share", Type: core.ConnectionTypeString, Label: "Destination Share", Placeholder: "my-share", Required: true},
	{Name: "directory", Type: core.ConnectionTypeString, Label: "Destination Directory", Placeholder: "Leave blank for the share's root, or e.g. backups/2026"},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "Destination File Name", Placeholder: "summary.pdf", Required: true},
	{Name: "source_share", Type: core.ConnectionTypeString, Label: "Source Share (same account)", Placeholder: "Copy from this account: source share name"},
	{Name: "source_path", Type: core.ConnectionTypeString, Label: "Source Path (same account)", Placeholder: "Copy from this account: source path, e.g. reports/2026/summary.pdf"},
	{Name: "source_url", Type: core.ConnectionTypeString, Label: "Source URL", Placeholder: "Or a full URL (public, or carrying its own SAS) — leave the two fields above blank"},
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "On by default: poll up to 30s until the copy succeeds or fails", Value: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the file is leased — the Lease ID output of a Lease File step"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Destination Path"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Copy Result"},
	{Name: "copy_status", Type: core.ConnectionTypeString, Label: "Copy Status"},
	{Name: "copy_id", Type: core.ConnectionTypeString, Label: "Copy ID"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

const (
	// pollInterval / pollBudget bound the wait_for_completion loop. Most
	// same-account copies finish instantly; a cross-account copy that outlives
	// the budget is reported as still pending, with the copy_id to check later.
	pollInterval = time.Second
	pollBudget   = 30 * time.Second
)

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

	sourceURL := files.OptionalString("source_url", inputs)
	srcShare := files.OptionalString("source_share", inputs)
	srcPath := strings.Trim(files.OptionalString("source_path", inputs), "/")
	switch {
	case sourceURL != "" && (srcShare != "" || srcPath != ""):
		return files.ErrorResult("provide EITHER source_url OR source_share + source_path, not both"), nil
	case sourceURL == "" && (srcShare == "" || srcPath == ""):
		return files.ErrorResult("provide source_url, or both source_share and source_path for a same-account copy"), nil
	case sourceURL == "":
		// A same-account source needs no SAS — the destination request's
		// authorization covers reading it.
		sourceURL = auth.BaseURL + files.DirectoryPath(srcShare, srcPath)
	default:
		if u, err := url.Parse(sourceURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return files.ErrorResult("source_url must be an http(s) URL"), nil
		}
	}

	destPath := files.FilePath(share, dir, fileName)
	headers := files.LeaseHeader(map[string]string{"x-ms-copy-source": sourceURL}, inputs)
	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodPut,
		Path:    destPath,
		Query:   url.Values{},
		Headers: headers,
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		if files.ErrorCode(resp) == "ParentNotFound" {
			return files.ErrorResult(fmt.Sprintf("the destination directory %q does not exist in share %q — create it first", dir, share)), nil
		}
		return files.ErrorResult(err.Error()), nil
	}

	copyID := resp.Headers.Get("x-ms-copy-id")
	status := resp.Headers.Get("x-ms-copy-status")
	statusDescription := ""

	if files.BoolDefaultTrue("wait_for_completion", inputs) {
		deadline := time.Now().Add(pollBudget)
		for status == "pending" && time.Now().Before(deadline) {
			select {
			case <-flow.GoContext().Done():
				return files.ErrorResult("copy polling cancelled"), nil
			case <-time.After(pollInterval):
			}
			head, err := files.Do(flow, auth, files.Request{
				Method:  http.MethodHead,
				Path:    destPath,
				Query:   url.Values{},
				Headers: files.LeaseHeader(nil, inputs),
			})
			if err != nil {
				return files.ErrorResult(err.Error()), nil
			}
			if err := files.CheckResponse(head); err != nil {
				return files.ErrorResult(err.Error()), nil
			}
			status = head.Headers.Get("x-ms-copy-status")
			statusDescription = head.Headers.Get("x-ms-copy-status-description")
		}
	}

	// The source is echoed REDACTED: a source_url carrying a SAS would
	// otherwise put a live sig= into the run record and every downstream node.
	result := map[string]interface{}{
		"path":       logical,
		"copyId":     copyID,
		"copyStatus": status,
		"source":     files.RedactURL(sourceURL),
	}
	if srcShare != "" {
		result["sourceShare"] = srcShare
		result["sourcePath"] = srcPath
	}
	if statusDescription != "" {
		result["copyStatusDescription"] = statusDescription
	}

	if status == "failed" || status == "aborted" {
		msg := fmt.Sprintf("copy to %s %s", logical, status)
		if statusDescription != "" {
			msg += ": " + statusDescription
		}
		return files.ErrorResult(msg), nil
	}

	out := files.ResourceResult(logical, result, fmt.Sprintf("Copied to %s in share %s (status %s)", logical, share, status))
	out["copy_status"] = status
	out["copy_id"] = copyID
	return out, nil
}
