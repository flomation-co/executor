package azure_storage_blob_download

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Download Blob"
	Description  = "Download a blob (optionally a byte range) to a file reference for downstream nodes. Text-like content up to 1 MB is also returned inline"
	Website      = "https://www.flomation.co"
	Icon         = "azure+arrow-down"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

// inlineContentLimit caps the inline `content` output; larger (or binary)
// blobs travel only as the file reference.
const inlineContentLimit = 1 << 20 // 1 MB

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Blob Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.blob.core.windows.net — leave blank to derive; Azurite: http://host:10000/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "my-container", Required: true},
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Blob Name", Placeholder: "reports/2026/summary.pdf", Required: true},
	{Name: "range", Type: core.ConnectionTypeString, Label: "Byte Range", Placeholder: "bytes=0-1023 — leave blank for the whole blob"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Blob"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File Reference"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content (text blobs ≤ 1 MB)"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// textLike reports whether a Content-Type is safe to surface as the inline
// string output.
func textLike(contentType string) bool {
	ct := strings.ToLower(strings.SplitN(contentType, ";", 2)[0])
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/json", "application/xml", "application/javascript",
		"application/x-ndjson", "application/csv":
		return true
	}
	return strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "+xml")
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

	headers := map[string]string{}
	if r := storage.OptionalString("range", inputs); r != "" {
		if !strings.HasPrefix(r, "bytes=") {
			return storage.ErrorResult(`range must look like "bytes=0-1023"`), nil
		}
		headers["Range"] = r
	}

	resp, err := storage.Do(flow, auth, storage.Request{
		Method:  http.MethodGet,
		Path:    storage.BlobPath(container, blobName),
		Query:   url.Values{},
		Headers: headers,
		MaxBody: storage.MaxDownloadBody, // blobs are big; the shared 8 MB default is for envelopes
	})
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if err := storage.CheckResponse(resp); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	contentType := resp.Headers.Get("Content-Type")

	// The bytes land on the workspace as a media file; EmitMediaFile hands
	// back a blob token for small files (previewable, survives suspension) or
	// a flo:file: reference for big ones — either is accepted downstream.
	scratch, err := flow.MediaScratchFile(path.Ext(blobName))
	if err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to allocate a scratch file: %s", err.Error())), nil
	}
	if err := os.WriteFile(scratch, resp.Body, 0o600); err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to write the download: %s", err.Error())), nil
	}
	fileRef, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to emit the download: %s", err.Error())), nil
	}

	inline := ""
	if len(resp.Body) <= inlineContentLimit && textLike(contentType) {
		inline = string(resp.Body)
	}

	out := storage.ResourceResult(blobName, storage.HeadersResult(blobName, resp.Headers),
		fmt.Sprintf("Downloaded %s from %s (%d bytes)", blobName, container, len(resp.Body)))
	out["file"] = fileRef
	out["content"] = inline
	out["content_type"] = contentType
	out["size"] = len(resp.Body)
	return out, nil
}
