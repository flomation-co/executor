package azure_storage_blob_download

import (
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
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
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the blob or container is leased — the Lease ID output of a Lease step"},
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

	opts := &blob.DownloadStreamOptions{}
	if r := storage.OptionalString("range", inputs); r != "" {
		httpRange, err := parseByteRange(r)
		if err != nil {
			return storage.ErrorResult(err.Error()), nil
		}
		opts.Range = httpRange
	}
	if lid := storage.LeaseIDPtr(inputs); lid != nil {
		opts.AccessConditions = &blob.AccessConditions{LeaseAccessConditions: &blob.LeaseAccessConditions{LeaseID: lid}}
	}

	bc, err := auth.BlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	resp, err := bc.DownloadStream(flow.GoContext(), opts)
	if err != nil {
		if storage.HasCode(err, bloberror.BlobNotFound) {
			return storage.ErrorResult(fmt.Sprintf("blob %q was not found in %q", blobName, container)), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Blobs are large; read up to the 256 MB ceiling and refuse (rather than
	// silently truncate) anything past it — the same guard the shared REST
	// client applied via MaxBody.
	body, err := io.ReadAll(io.LimitReader(resp.Body, storage.MaxDownloadBody+1))
	if err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to read the download: %s", err.Error())), nil
	}
	if int64(len(body)) > storage.MaxDownloadBody {
		return storage.ErrorResult(fmt.Sprintf("blob %q exceeds the %d MB download ceiling", blobName, storage.MaxDownloadBody>>20)), nil
	}

	contentType := ""
	if resp.ContentType != nil {
		contentType = *resp.ContentType
	}

	// The bytes land on the workspace as a media file; EmitMediaFile hands
	// back a blob token for small files (previewable, survives suspension) or
	// a flo:file: reference for big ones — either is accepted downstream.
	scratch, err := flow.MediaScratchFile(path.Ext(blobName))
	if err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to allocate a scratch file: %s", err.Error())), nil
	}
	if err := os.WriteFile(scratch, body, 0o600); err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to write the download: %s", err.Error())), nil
	}
	fileRef, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to emit the download: %s", err.Error())), nil
	}

	inline := ""
	if len(body) <= inlineContentLimit && textLike(contentType) {
		inline = string(body)
	}

	props := map[string]interface{}{}
	if contentType != "" {
		props["contentType"] = contentType
	}
	if resp.ContentLength != nil {
		props["contentLength"] = *resp.ContentLength
	}
	// Content-Encoding is a STORED property of the blob, not transfer encoding —
	// a gzip-encoded blob must report it so downstream knows the bytes are
	// compressed. The shared client sets DisableCompression precisely so
	// net/http never strips it. (Dropping these was a real regression the live
	// round-trip caught.)
	if resp.ContentEncoding != nil {
		props["contentEncoding"] = *resp.ContentEncoding
	}
	if resp.ContentLanguage != nil {
		props["contentLanguage"] = *resp.ContentLanguage
	}
	if resp.ContentDisposition != nil {
		props["contentDisposition"] = *resp.ContentDisposition
	}
	if resp.CacheControl != nil {
		props["cacheControl"] = *resp.CacheControl
	}
	if resp.ETag != nil {
		props["etag"] = string(*resp.ETag)
	}
	if resp.LastModified != nil {
		props["lastModified"] = resp.LastModified.UTC().Format(time.RFC1123)
	}
	out := storage.PropsResult(blobName,
		fmt.Sprintf("Downloaded %s from %s (%d bytes)", blobName, container, len(body)),
		props, storage.StrMeta(resp.Metadata))
	out["file"] = fileRef
	out["content"] = inline
	out["content_type"] = contentType
	out["size"] = len(body)
	return out, nil
}

// parseByteRange turns an HTTP byte-range spec ("bytes=0-1023", "bytes=500-")
// into the SDK's blob.HTTPRange. A missing end means "to the end of the blob"
// (Count stays 0). This preserves the action's original "bytes=..." input form.
func parseByteRange(r string) (blob.HTTPRange, error) {
	var out blob.HTTPRange
	if !strings.HasPrefix(r, "bytes=") {
		return out, fmt.Errorf(`range must look like "bytes=0-1023"`)
	}
	spec := strings.TrimPrefix(r, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	start, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || start < 0 {
		return out, fmt.Errorf(`range must look like "bytes=0-1023"`)
	}
	out.Offset = start
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		end, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || end < start {
			return out, fmt.Errorf(`range end must be a number no smaller than the start (e.g. "bytes=0-1023")`)
		}
		out.Count = end - start + 1
	}
	// An open-ended spec ("bytes=500-") leaves Count at its zero value, which
	// the SDK reads as "from Offset to the end of the blob" — not "zero bytes".
	return out, nil
}
