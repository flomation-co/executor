package azure_files_file_upload

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Upload File"
	Description  = "Upload content to a file in a share — inline text, base64, a flo:file reference, or an uploaded asset. Azure Files writes in two steps (allocate, then write the bytes in 4 MiB ranges), which this step does for you. Content is capped at 256 MB"
	Website      = "https://www.flomation.co"
	Icon         = "azure+arrow-up"
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
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "Text, base64, a flo:file reference, or ${parent.file} from an upstream node", Required: true},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "Detected from the content when blank (e.g. application/pdf)"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"source":"flomation"}`},
	{Name: "overwrite", Type: core.ConnectionTypeBoolean, Label: "Overwrite", Placeholder: "On by default. Turn off to fail instead of replacing an existing file", Value: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the file is leased — the Lease ID output of a Lease File step"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "File Path"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "File"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "File URL"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Execute performs the TWO-STEP write that separates Azure Files from Blob
// Storage, and it is the sharpest thing in this node.
//
//	Step 1 — Create File: PUT with x-ms-type: file and x-ms-content-length: N.
//	         This ALLOCATES a sparse file of N bytes and writes NONE of them.
//	         On its own it succeeds and leaves an N-byte file full of zeros.
//	Step 2 — Put Range: PUT ?comp=range with x-ms-write: update and a Range
//	         header, at most MaxRangeBytes per call (413 above it). This is
//	         where the bytes actually land, so anything larger is a LOOP.
//
// Two consequences the code has to answer for:
//
//   - Put Range cannot create. "Calling Put Range with a file name that doesn't
//     currently exist returns status code 404" — so the order is fixed and
//     step 1 is not optional.
//   - The pair is NOT atomic. A Create that succeeds followed by a Put Range
//     that fails leaves a correctly sized, zero-filled file behind — a
//     corruption Blob's single Put Blob cannot produce, and one that reads as
//     success to anything that only checks the size. A failed write therefore
//     deletes the file it allocated before reporting, and says so.
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
	if err := files.ValidateFilePath("file_name", logical); err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil || *contentConn.String() == "" {
		return files.ErrorResult("content is required"), nil
	}
	// ResolveToBytes accepts every inbound media form: a flo:file: workspace
	// reference, a flo:blob: token, base64, or raw text.
	body, resolvedMime, err := flow.ResolveToBytes(*contentConn.String())
	if err != nil {
		return files.ErrorResult(fmt.Sprintf("failed to resolve content: %s", err.Error())), nil
	}
	if len(body) > files.MaxUploadBody {
		return files.ErrorResult(fmt.Sprintf("the content is %d bytes, over the %d MB upload limit — use Copy File, which transfers server-side at any size",
			len(body), files.MaxUploadBody>>20)), nil
	}

	contentType := files.OptionalString("content_type", inputs)
	if contentType == "" {
		contentType = resolvedMime
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Step 1 — allocate. x-ms-content-type carries the file's stored type here:
	// the request's own Content-Type describes a body that does not exist yet.
	createHeaders := map[string]string{
		"x-ms-type":           "file",
		"x-ms-content-length": strconv.Itoa(len(body)),
		"x-ms-content-type":   contentType,
	}
	if err := files.MetadataHeaders(createHeaders, inputs, "metadata"); err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if !files.BoolDefaultTrue("overwrite", inputs) {
		// If-None-Match: * makes the create conditional on absence — the
		// service answers 409/412 instead of replacing.
		createHeaders["If-None-Match"] = "*"
	}
	createHeaders = files.LeaseHeader(createHeaders, inputs)

	path := files.FilePath(share, dir, fileName)
	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodPut,
		Path:    path,
		Query:   url.Values{},
		Headers: createHeaders,
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		switch files.ErrorCode(resp) {
		case "ResourceAlreadyExists", "ConditionNotMet":
			return files.ErrorResult(fmt.Sprintf("file %q already exists in share %q and Overwrite is off", logical, share)), nil
		case "ParentNotFound":
			return files.ErrorResult(fmt.Sprintf("the directory %q does not exist in share %q — Azure Files has no implicit directories, so create it first", dir, share)), nil
		}
		return files.ErrorResult(err.Error()), nil
	}

	// Step 2 — write. A zero-byte file is complete at step 1: there is no range
	// to put, and Put Range with an empty body is rejected.
	etag := resp.Headers.Get("ETag")
	for offset := 0; offset < len(body); offset += files.MaxRangeBytes {
		end := offset + files.MaxRangeBytes
		if end > len(body) {
			end = len(body)
		}
		rangeHeaders := map[string]string{
			"x-ms-write": "update",
			"Range":      fmt.Sprintf("bytes=%d-%d", offset, end-1),
		}
		rangeHeaders = files.LeaseHeader(rangeHeaders, inputs)

		rangeResp, err := files.Do(flow, auth, files.Request{
			Method:  http.MethodPut,
			Path:    path,
			Query:   url.Values{"comp": []string{"range"}},
			Headers: rangeHeaders,
			Body:    body[offset:end],
		})
		if err != nil {
			return files.ErrorResult(cleanupFailedWrite(flow, auth, path, inputs, err.Error())), nil
		}
		if err := files.CheckResponse(rangeResp); err != nil {
			return files.ErrorResult(cleanupFailedWrite(flow, auth, path, inputs, err.Error())), nil
		}
		etag = rangeResp.Headers.Get("ETag")
	}

	result := files.HeadersResult(logical, resp.Headers)
	result["path"] = logical
	result["size"] = len(body)
	ranges := (len(body) + files.MaxRangeBytes - 1) / files.MaxRangeBytes
	result["ranges"] = ranges

	summary := fmt.Sprintf("Uploaded %s to share %s (%d bytes)", logical, share, len(body))
	if ranges > 1 {
		summary += fmt.Sprintf(" in %d ranges", ranges)
	}
	out := files.ResourceResult(logical, result, summary)
	out["etag"] = etag
	out["url"] = auth.BaseURL + path
	out["size"] = len(body)
	return out, nil
}

// cleanupFailedWrite removes the sparse file a failed Put Range would otherwise
// leave behind, and folds the outcome into the error the operator sees. Without
// it, a half-written upload leaves a file of exactly the right SIZE and none of
// the right bytes — which every size check downstream calls a success.
//
// Best-effort by construction: if the delete also fails (the same transport
// problem, most likely) the message says the file is still there rather than
// claiming a cleanliness it cannot verify.
func cleanupFailedWrite(flow *core.Flow, auth files.Auth, path string, inputs []*core.Connection, msg string) string {
	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodDelete,
		Path:    path,
		Query:   url.Values{},
		Headers: files.LeaseHeader(nil, inputs),
	})
	if err == nil && files.CheckResponse(resp) == nil {
		return "failed to write the file's content: " + msg + " — the empty file that had been allocated was removed"
	}
	return "failed to write the file's content: " + msg +
		" — WARNING: an allocated, zero-filled file of the right size was left behind and could not be removed; delete it before anything reads it"
}
