// Package crm_salesforce_attachment_download fetches the actual bytes of a
// Classic Attachment.
//
// n8n cannot do this at all. Its attachment read returns the record, whose Body
// field is a URL PATH rather than the file — so a flow that looks like it is
// forwarding an invoice is actually forwarding the string
// "/services/data/v62.0/sobjects/Attachment/00P…/Body". Fetching that path is
// what this action does.
//
// The bytes stream straight to the execution workspace instead of being
// buffered, and the read is bounded so an oversized attachment is REFUSED
// rather than silently clipped — the shared client's response reader truncates
// past its cap, which on a download hands back a corrupt file that looks fine.
package crm_salesforce_attachment_download

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
	Name         = "Salesforce: Download Attachment (Classic)"
	Description  = "Download a Classic attachment's contents so a later step can email it, save it to Drive or send it on."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+file-arrow-down"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

const (
	// maxDownloadBytes comfortably clears Salesforce's own 25 MB attachment
	// ceiling; the headroom is there so a malformed response is caught rather
	// than streamed to disk forever.
	maxDownloadBytes = 64 << 20 // 64 MB

	// downloadTimeout replaces the shared client's 60s budget: a large
	// attachment on a slow link legitimately takes minutes.
	downloadTimeout = 10 * time.Minute
)

var downloadClient = &http.Client{Timeout: downloadTimeout}

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment", Placeholder: "Attachment ID, e.g. 00P5f00000XyzAAB", Required: true},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "Save As", Placeholder: "Leave blank to use the name the attachment has in Salesforce"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "Downloaded File"},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Attachment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	attachmentID, err := salesforce.RequiredString("attachment_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("attachment_id is required — the attachment whose contents you want")
	}
	if err := salesforce.ValidateRecordID(attachmentID); err != nil {
		return nil, err
	}

	// Read the record first for the stored name and type. The blob endpoint
	// alone gives neither, and a downloaded file with no extension is close to
	// useless to every step after this one.
	record, err := salesforce.GetRecord(instanceURL, token, "Attachment", attachmentID,
		"Id,Name,ContentType,BodyLength,Description,ParentId,OwnerId,IsPrivate,CreatedDate")
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	name := salesforce.OptionalString("file_name", inputs)
	if name == "" {
		if stored, _ := record["Name"].(string); stored != "" {
			name = stored
		} else {
			name = attachmentID
		}
	}

	scratch, err := flow.MediaScratchFile(filepath.Ext(name))
	if err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to allocate a working file: %v", err)), nil
	}

	size, contentType, err := streamTo(instanceURL, token, "/sobjects/Attachment/"+attachmentID+"/Body", scratch)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if contentType == "" {
		// Salesforce echoes the stored ContentType on the blob response, but
		// fall back to the record's own value rather than emit nothing.
		contentType, _ = record["ContentType"].(string)
	}

	// EmitMediaFile hands back a blob token for a small file (previewable in
	// the run history, survives a Wait/approval suspension) or a workspace
	// reference for a large one. Downstream actions accept either.
	ref, err := flow.EmitMediaFile(scratch)
	if err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to hand on the downloaded file: %v", err)), nil
	}

	out := salesforce.RecordResult(attachmentID, record, fmt.Sprintf("Downloaded %s (%d bytes)", name, size))
	out["file"] = ref
	out["file_name"] = name
	out["content_type"] = contentType
	out["size"] = size
	return out, nil
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
		return 0, "", fmt.Errorf("the attachment is larger than the %d MB download limit", int64(maxDownloadBytes)>>20)
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
