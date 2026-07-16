package azure_storage_blob_copy

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Copy Blob"
	Description  = "Start a server-side (async) blob copy — from another blob in the same account, or from any URL Azure can reach — and optionally wait for it to complete. Handles blobs of any size, unlike Upload from URL's 256 MB synchronous cap"
	Website      = "https://www.flomation.co"
	Icon         = "azure+copy"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

const (
	// pollInterval / pollBudget bound the wait_for_completion loop. Most
	// same-account copies finish instantly (the service dedups by content); a
	// cross-account copy that outlives the budget is reported as still
	// pending, with the copy_id to check later.
	pollInterval = time.Second
	pollBudget   = 30 * time.Second
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Destination Container", Placeholder: "my-container", Required: true},
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Destination Blob Name", Placeholder: "backups/summary.pdf", Required: true},
	{Name: "source_container", Type: core.ConnectionTypeString, Label: "Source Container (same account)", Placeholder: "Copy from this account: source container name"},
	{Name: "source_blob", Type: core.ConnectionTypeString, Label: "Source Blob (same account)", Placeholder: "Copy from this account: source blob name"},
	{Name: "source_url", Type: core.ConnectionTypeString, Label: "Source URL", Placeholder: "Or a full URL (public, or carrying its own SAS) — leave the two fields above blank"},
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "On by default: poll up to 30s until the copy succeeds or fails", Value: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Destination Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Copy Result"},
	{Name: "copy_status", Type: core.ConnectionTypeString, Label: "Copy Status"},
	{Name: "copy_id", Type: core.ConnectionTypeString, Label: "Copy ID"},
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
	blobName, err := storage.RequiredString("blob_name", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	sourceURL := storage.OptionalString("source_url", inputs)
	srcContainer := storage.OptionalString("source_container", inputs)
	srcBlob := storage.OptionalString("source_blob", inputs)
	switch {
	case sourceURL != "" && (srcContainer != "" || srcBlob != ""):
		return storage.ErrorResult("provide EITHER source_url OR source_container + source_blob, not both"), nil
	case sourceURL == "" && (srcContainer == "" || srcBlob == ""):
		return storage.ErrorResult("provide source_url, or both source_container and source_blob for a same-account copy"), nil
	case sourceURL == "":
		// A same-account source needs no SAS — the destination request's
		// authorization covers reading it.
		sourceURL = auth.BaseURL + storage.BlobPath(srcContainer, srcBlob)
	default:
		if u, err := url.Parse(sourceURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return storage.ErrorResult("source_url must be an http(s) URL"), nil
		}
	}

	destPath := storage.BlobPath(container, blobName)
	resp, err := storage.Do(flow, auth, storage.Request{
		Method:  http.MethodPut,
		Path:    destPath,
		Query:   url.Values{},
		Headers: map[string]string{"x-ms-copy-source": sourceURL},
	})
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if err := storage.CheckResponse(resp); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	copyID := resp.Headers.Get("x-ms-copy-id")
	status := resp.Headers.Get("x-ms-copy-status")
	statusDescription := ""

	if storage.BoolDefaultTrue("wait_for_completion", inputs) {
		deadline := time.Now().Add(pollBudget)
		for status == "pending" && time.Now().Before(deadline) {
			select {
			case <-flow.GoContext().Done():
				return storage.ErrorResult("copy polling cancelled"), nil
			case <-time.After(pollInterval):
			}
			head, err := storage.Do(flow, auth, storage.Request{
				Method: http.MethodHead,
				Path:   destPath,
				Query:  url.Values{},
			})
			if err != nil {
				return storage.ErrorResult(err.Error()), nil
			}
			if err := storage.CheckResponse(head); err != nil {
				return storage.ErrorResult(err.Error()), nil
			}
			status = head.Headers.Get("x-ms-copy-status")
			statusDescription = head.Headers.Get("x-ms-copy-status-description")
		}
	}

	// The source is echoed REDACTED: a source_url carrying a SAS would
	// otherwise put a live sig= into the run record and every downstream node.
	result := map[string]interface{}{
		"copyId":     copyID,
		"copyStatus": status,
		"source":     storage.RedactURL(sourceURL),
	}
	if srcContainer != "" {
		result["sourceContainer"] = srcContainer
		result["sourceBlob"] = srcBlob
	}
	if statusDescription != "" {
		result["copyStatusDescription"] = statusDescription
	}

	if status == "failed" || status == "aborted" {
		msg := fmt.Sprintf("copy to %s %s", blobName, status)
		if statusDescription != "" {
			msg += ": " + statusDescription
		}
		return storage.ErrorResult(msg), nil
	}

	summary := fmt.Sprintf("Copied to %s in %s (status %s)", blobName, container, status)
	out := storage.ResourceResult(blobName, result, summary)
	out["copy_status"] = status
	out["copy_id"] = copyID
	return out, nil
}
