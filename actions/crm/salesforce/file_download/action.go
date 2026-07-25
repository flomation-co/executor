// Package crm_salesforce_file_download fetches the actual bytes of a Salesforce
// file so they can be pushed somewhere else — Drive, SharePoint, an email
// attachment, a signature service.
//
// n8n has no download path at all: its file actions can create records and read
// metadata, but the bytes are simply unreachable, which makes "get the signed
// contract out of Salesforce" impossible without dropping to a raw HTTP call.
//
// Two details make this its own file rather than a call to the shared client.
// The bytes are streamed straight to the execution workspace instead of being
// buffered, and the shared client's response reader caps bodies at 8 MB and
// TRUNCATES silently past it — which on a download would hand the operator a
// corrupt file that looks like a success.
package crm_salesforce_file_download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Download File"
	Description  = "Download a Salesforce file's contents so a later step can email it, save it to Drive or send it on. Accepts a file ID or a specific version ID."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+file-arrow-down"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

const (
	// maxDownloadBytes bounds a single download. Refusing past the ceiling is
	// the point — a silently truncated file is worse than no file.
	maxDownloadBytes = 512 << 20 // 512 MB

	// downloadTimeout replaces the shared client's 60s budget: a large file on
	// a slow link legitimately takes minutes.
	downloadTimeout = 30 * time.Minute

	// contentDocumentPrefix is the Salesforce key prefix for a ContentDocument
	// (the file) as opposed to 068 for a ContentVersion (one revision of it).
	// Operators copy whichever ID is in front of them, so both are accepted.
	contentDocumentPrefix = "069"
)

var downloadClient = &http.Client{Timeout: downloadTimeout}

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File", Placeholder: "File ID (069…) or version ID (068…) from an upload or files list", Required: true},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "Save As", Placeholder: "Leave blank to use the name the file has in Salesforce"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "File Version ID"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "Downloaded File"},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "File"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	fileID, err := salesforce.RequiredString("file_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("file_id is required — the File ID from an upload step or a files list")
	}
	if err := salesforce.ValidateRecordID(fileID); err != nil {
		return nil, err
	}

	versionID, record, err := resolveVersion(instanceURL, token, fileID)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Give the scratch file the real extension so downstream MIME sniffing,
	// previews and "attach this to an email" all behave.
	name := downloadName(salesforce.OptionalString("file_name", inputs), record, versionID)
	scratch, err := flow.MediaScratchFile(filepath.Ext(name))
	if err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to allocate a working file: %v", err)), nil
	}

	size, contentType, err := streamTo(instanceURL, token, "/sobjects/ContentVersion/"+versionID+"/VersionData", scratch)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// EmitMediaFile hands back a blob token for a small file (previewable in
	// the run history, survives a Wait/approval suspension) or a workspace
	// reference for a large one. Downstream actions accept either.
	ref, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to hand on the downloaded file: %v", err)), nil
	}

	out := salesforce.RecordResult(versionID, record, fmt.Sprintf("Downloaded %s (%d bytes)", name, size))
	out["file"] = ref
	out["file_name"] = name
	out["content_type"] = contentType
	out["size"] = size
	return out, nil
}

// resolveVersion turns whichever ID the operator had to hand into the version
// ID that actually holds the bytes, and returns the file's metadata with it.
//
// A ContentDocument (069…) is the file; a ContentVersion (068…) is one revision
// of it. Only versions carry VersionData, so a document ID has to be followed
// to its latest published version first — a step nobody outside Salesforce
// knows they need.
func resolveVersion(instanceURL, token, fileID string) (string, map[string]interface{}, error) {
	if strings.HasPrefix(fileID, contentDocumentPrefix) {
		doc, err := salesforce.GetRecord(instanceURL, token, "ContentDocument", fileID,
			"Id,Title,FileExtension,FileType,ContentSize,LatestPublishedVersionId,OwnerId,CreatedDate")
		if err != nil {
			return "", nil, err
		}
		versionID := salesforce.StringifyID(doc["LatestPublishedVersionId"])
		if versionID == "" {
			return "", nil, fmt.Errorf("file %s has no published version to download — it may still be uploading, or the connected user cannot see it", fileID)
		}
		return versionID, doc, nil
	}

	version, err := salesforce.GetRecord(instanceURL, token, "ContentVersion", fileID,
		"Id,Title,PathOnClient,FileExtension,FileType,ContentSize,ContentDocumentId,VersionNumber,OwnerId,CreatedDate")
	if err != nil {
		return "", nil, err
	}
	return fileID, version, nil
}

// downloadName picks the name to save under: the operator's override, then the
// original client path (which keeps the extension), then the title plus the
// stored extension, then the ID as a last resort.
func downloadName(override string, record map[string]interface{}, versionID string) string {
	if override != "" {
		return override
	}
	if path, _ := record["PathOnClient"].(string); path != "" {
		return path
	}
	title, _ := record["Title"].(string)
	ext, _ := record["FileExtension"].(string)
	switch {
	case title != "" && ext != "" && !strings.HasSuffix(strings.ToLower(title), "."+strings.ToLower(ext)):
		return title + "." + ext
	case title != "":
		return title
	default:
		return versionID
	}
}

// streamTo copies a Salesforce blob endpoint straight to a file on disk.
//
// The read is bounded at maxDownloadBytes+1 so an oversized file is DETECTED
// rather than quietly clipped: copying exactly the ceiling and stopping would
// produce a plausible-looking but broken file.
func streamTo(instanceURL, token, path, dest string) (int64, string, error) {
	req, err := http.NewRequest(http.MethodGet, salesforce.BuildURL(instanceURL, path), nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create the download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// Salesforce answers a blob endpoint with the file's own content type; the
	// JSON Accept the shared client sends is only right for record endpoints.
	req.Header.Set("Accept", "*/*")

	resp, err := downloadClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("Salesforce download failed: %w", redact(err, token))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// A failure body is JSON and small; reuse the shared error translation
		// so the operator gets "the record no longer exists" rather than a code.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return 0, "", salesforce.CheckResponse(&salesforce.APIResponse{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header})
	}

	// #nosec G304 -- dest is our own MediaScratchFile result: a random name
	// inside the execution workspace's media scratch directory, never operator
	// input.
	fh, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, "", fmt.Errorf("failed to open a working file: %w", err)
	}
	written, copyErr := io.Copy(fh, io.LimitReader(resp.Body, maxDownloadBytes+1))
	closeErr := fh.Close()
	if copyErr != nil {
		return 0, "", fmt.Errorf("failed to save the download: %w", copyErr)
	}
	if closeErr != nil {
		return 0, "", fmt.Errorf("failed to save the download: %w", closeErr)
	}
	if written > maxDownloadBytes {
		return 0, "", fmt.Errorf("the file is larger than the %d MB download limit", int64(maxDownloadBytes)>>20)
	}
	return written, strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0], nil
}

// redact strips the access token out of a transport error, which can quote the
// request URL and — after a misbehaving proxy or redirect — the credential.
func redact(err error, token string) error {
	if err == nil || token == "" || !strings.Contains(err.Error(), token) {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), token, "********"))
}
