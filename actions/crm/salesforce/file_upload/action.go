// Package crm_salesforce_file_upload uploads a file into Salesforce Files.
//
// This is the file action that actually works in a modern org. Salesforce has
// two file stores: Classic Attachments (attachment_create) and Files —
// ContentVersion/ContentDocument, the thing the Lightning "Files" related list
// shows. n8n's file support writes Attachments, so a signed contract pushed by
// an n8n flow lands somewhere the person who needed it never looks. Files is
// the default here for exactly that reason.
//
// The upload is multipart/form-data rather than a JSON body carrying base64
// VersionData. Both work, but the JSON route is capped around 50 MB of encoded
// text (roughly a 37 MB file, since base64 inflates by a third) and it forces
// the entire file into memory twice. Multipart streams straight off the
// workspace file with no encoding step, so a 300 MB video costs the same
// memory as a 30 KB PDF.
package crm_salesforce_file_upload

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	Name         = "Salesforce: Upload File"
	Description  = "Upload a file to Salesforce Files and optionally attach it to a record, so it shows up in the Files related list your team actually looks at."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+cloud-arrow-up"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxFileBytes is Salesforce's ceiling for a file uploaded through the REST
// multipart route. Checking it here turns a slow upload that ends in an opaque
// 400 into an immediate, specific message.
const maxFileBytes = 2 << 30 // 2 GB

// uploadTimeout replaces the shared client's 60s budget: a large file over a
// slow link legitimately takes minutes, and a timeout mid-upload leaves the
// operator with no record and no explanation.
const uploadTimeout = 30 * time.Minute

// maxUploadResponse caps the response read. The create response is a handful of
// bytes ({"id":…,"success":true,"errors":[]}); an error page is the only thing
// that could be large.
const maxUploadResponse = 1 << 20 // 1 MB

// uploadClient is separate from the shared salesforce client purely for the
// longer timeout — everything else about the request is identical.
var uploadClient = &http.Client{Timeout: uploadTimeout}

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "title", Type: core.ConnectionTypeString, Label: "File Title", Placeholder: "Signed contract — Acme Ltd", Required: true},

	// Accepts anything an earlier step produced (a generated PDF, a downloaded
	// attachment, a rendered chart) as well as pasted base64, because every
	// media-producing action in Flomation emits one of those three forms.
	{Name: "file", Type: core.ConnectionTypeString, Label: "File", Placeholder: "The file from an earlier step, or base64 content", Required: true},

	// PathOnClient is what Salesforce reads the file type from — get it wrong
	// and a PDF is stored as an unknown blob with no preview.
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name", Placeholder: "contract.pdf — the extension sets the file type; blank uses the title"},

	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What this file is, for whoever finds it later"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "File Owner", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB — blank means you"},

	// link_to_object never reaches Salesforce: FirstPublishLocationId is just a
	// record ID. It is here so the editor can narrow the record picker below to
	// one object, and so the result summary can say what the file was attached
	// to in plain English.
	{Name: "link_to_object", Type: core.ConnectionTypeString, Label: "Attach To (record type)", Placeholder: "Contact, Account, Opportunity… — only used to help you pick the record"},
	{Name: "link_to_object_id", Type: core.ConnectionTypeString, Label: "Attach To Record", Placeholder: "Record ID the file belongs to, e.g. 0035f00000XyzAAB"},

	// Every org has custom fields on ContentVersion too (document type,
	// retention date, matter reference), so the escape hatch is normal use.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Document_Type__c":"Contract"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "File Version ID"},
	// The document ID, not the version ID, is what every other file action
	// takes (share, delete, list). Surfacing it here saves the operator a
	// lookup step they would otherwise have to know they needed.
	{Name: "content_document_id", Type: core.ConnectionTypeString, Label: "File ID (ContentDocument)"},
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

	title, err := salesforce.RequiredString("title", inputs)
	if err != nil {
		return nil, fmt.Errorf("title is required — Salesforce shows it as the file's name in the Files list")
	}
	source, err := salesforce.RequiredString("file", inputs)
	if err != nil {
		return nil, fmt.Errorf("file is required — wire in a file from an earlier step, or paste base64 content")
	}

	// Landing the bytes on the workspace first is what makes the upload
	// streamable: a flo:file: reference is used in place with no copy, a blob
	// token or base64 string is written to scratch once. Nothing is ever held
	// in memory in full.
	path, _, err := flow.ResolveToLocalFile(source)
	if err != nil {
		return nil, fmt.Errorf("could not read the file: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("could not read the file: %w", err)
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("the file is %d MB — Salesforce Files accepts up to %d MB per upload", info.Size()>>20, int64(maxFileBytes)>>20)
	}

	fileName := resolveFileName(salesforce.OptionalString("file_name", inputs), title, path)

	// ContentLocation "S" means Salesforce-hosted. The alternatives ("E"
	// external, "L" link) describe files that live somewhere else entirely and
	// have nothing to do with uploading bytes, so they are not exposed.
	entity := map[string]interface{}{
		"Title":           title,
		"PathOnClient":    fileName,
		"ContentLocation": "S",
	}
	salesforce.SetIfPresent(entity, inputs, "Description", "description")
	salesforce.SetIfPresent(entity, inputs, "OwnerId", "owner_id")
	// FirstPublishLocationId is create-only: it is the one chance to file the
	// document against a record without a second ContentDocumentLink call.
	salesforce.SetIfPresent(entity, inputs, "FirstPublishLocationId", "link_to_object_id")
	if err := salesforce.MergeAdditionalFields(entity, inputs); err != nil {
		return nil, err
	}

	resp, err := postContentVersion(instanceURL, token, path, fileName, entity)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var created struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce upload response: %v", err)), nil
	}
	if created.ID == "" {
		return salesforce.ErrorResult("Salesforce accepted the upload but did not return a file ID"), nil
	}

	// Read the stored version back so the operator gets real metadata (size,
	// extension, document ID) rather than the three-field create envelope.
	// Best-effort: the file IS uploaded at this point, and failing the whole
	// action over a follow-up read would be a lie about what happened.
	record, docID := describeVersion(instanceURL, token, created.ID)

	out := salesforce.RecordResult(created.ID, record, uploadSummary(title, fileName, info.Size(), inputs))
	out["content_document_id"] = docID
	return out, nil
}

// resolveFileName settles on the PathOnClient value Salesforce reads the file
// type from. An explicit file name wins; otherwise the title is given the
// resolved file's extension, because a title of "Signed contract" stored with
// no extension previews as an unknown blob.
func resolveFileName(explicit, title, path string) string {
	if explicit != "" {
		return explicit
	}
	if ext := filepath.Ext(path); ext != "" {
		return title + ext
	}
	return title
}

// uploadSummary phrases the outcome the way the operator described the job:
// what was uploaded, how big it was, and what it is now attached to.
func uploadSummary(title, fileName string, size int64, inputs []*core.Connection) string {
	summary := fmt.Sprintf("Uploaded %s as %s (%d bytes)", title, fileName, size)
	if linked := salesforce.OptionalString("link_to_object_id", inputs); linked != "" {
		object := salesforce.OptionalString("link_to_object", inputs)
		if object == "" {
			object = "record"
		}
		summary += fmt.Sprintf(" and attached it to %s %s", object, linked)
	}
	return summary
}

// describeVersion reads the uploaded ContentVersion back. Errors are swallowed
// deliberately — the caller treats this as decoration on a completed upload.
func describeVersion(instanceURL, token, versionID string) (map[string]interface{}, string) {
	record, err := salesforce.GetRecord(instanceURL, token, "ContentVersion", versionID,
		"Id,Title,PathOnClient,FileExtension,FileType,ContentSize,ContentDocumentId,VersionNumber,OwnerId,CreatedDate")
	if err != nil || record == nil {
		return map[string]interface{}{"Id": versionID}, ""
	}
	return record, salesforce.StringifyID(record["ContentDocumentId"])
}

// postContentVersion streams the file to Salesforce as multipart/form-data.
//
// The shared ExecuteAPI helper only speaks JSON, and its response reader caps
// bodies — neither is a fit for a blob upload — so this is the one place the
// node builds its own request. The two parts and their names are Salesforce's
// blob-insert convention, not ours: "entity_content" carries the field map as
// JSON and MUST declare application/json, and the binary part MUST be named
// after the blob field ("VersionData") and carry a filename.
func postContentVersion(instanceURL, token, path, fileName string, entity map[string]interface{}) (*salesforce.APIResponse, error) {
	entityJSON, err := json.Marshal(entity)
	if err != nil {
		return nil, fmt.Errorf("failed to build the upload payload: %w", err)
	}
	// #nosec G304 -- path is ResolveToLocalFile's own result, which is confined
	// to the execution workspace (traversal and symlink escapes are rejected
	// there); it is never a raw operator string.
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not read the file: %w", err)
	}

	// io.Pipe keeps the request body a stream: the goroutine assembles the
	// multipart form while net/http reads it, so the file never has to be
	// materialised as one buffer. The boundary is captured before the goroutine
	// starts so the Content-Type header is set from the calling goroutine.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	go func() {
		defer func() { _ = fh.Close() }()
		if err := writeUploadParts(mw, entityJSON, fh, fileName); err != nil {
			// CloseWithError surfaces the failure on the reading side, so the
			// HTTP call aborts with our message instead of sending a truncated
			// body that Salesforce would reject with something unhelpful.
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	req, err := http.NewRequest(http.MethodPost, salesforce.BuildURL(instanceURL, "/sobjects/ContentVersion"), pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		return nil, fmt.Errorf("failed to create the upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)

	resp, err := uploadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Salesforce upload failed: %w", redact(err, token))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUploadResponse))
	if err != nil {
		return nil, fmt.Errorf("failed to read the Salesforce response: %w", err)
	}
	return &salesforce.APIResponse{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}

// writeUploadParts assembles the two-part form. Part headers are built by hand
// rather than with CreateFormFile because the JSON part needs an explicit
// application/json content type, which CreateFormField does not set.
func writeUploadParts(mw *multipart.Writer, entityJSON []byte, file io.Reader, fileName string) error {
	meta := make(textproto.MIMEHeader)
	meta.Set("Content-Disposition", `form-data; name="entity_content"`)
	meta.Set("Content-Type", "application/json")
	metaPart, err := mw.CreatePart(meta)
	if err != nil {
		return err
	}
	if _, err := metaPart.Write(entityJSON); err != nil {
		return err
	}

	blob := make(textproto.MIMEHeader)
	blob.Set("Content-Disposition", `form-data; name="VersionData"; filename="`+escapeHeaderValue(fileName)+`"`)
	blob.Set("Content-Type", "application/octet-stream")
	blobPart, err := mw.CreatePart(blob)
	if err != nil {
		return err
	}
	if _, err := io.Copy(blobPart, file); err != nil {
		return err
	}
	return mw.Close()
}

// escapeHeaderValue makes a file name safe inside a quoted Content-Disposition
// parameter. A quote or backslash in a file name would otherwise terminate the
// parameter early and corrupt the whole multipart envelope.
func escapeHeaderValue(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", "", "\n", "").Replace(s)
}

// redact strips the access token out of a transport error, which can quote the
// request URL and — after a misbehaving proxy or redirect — the credential.
func redact(err error, token string) error {
	if err == nil || token == "" || !strings.Contains(err.Error(), token) {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), token, "********"))
}
