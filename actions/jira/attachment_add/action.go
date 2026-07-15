package jira_attachment_add

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Add Attachment"
	Description  = "Attach a file to a Jira issue. Provide the issue key, a file name (e.g. report.pdf) and the file's bytes as base64 — Flomation uploads it to the issue. Returns the created attachment's details."
	Website      = "https://www.flomation.co"
	Icon         = "jira+paperclip"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue to attach the file to, e.g. SCRUM-1", Required: true},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name", Placeholder: "The name to store the file under, e.g. report.pdf", Required: true},
	{Name: "base64_content", Type: core.ConnectionTypeString, Label: "File Content (base64)", Placeholder: "The file's bytes, base64-encoded", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Attachment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := jira.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	key, err := jira.RequiredString("issue_key", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	fileName, err := jira.RequiredString("file_name", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	b64, err := jira.RequiredString("base64_content", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	var raw []byte
	if core.IsFileRef(b64) || core.IsBlobToken(b64) {
		raw, _, err = flow.ResolveToBytes(b64)
		if err != nil {
			return jira.ErrorResult("could not read the attachment: " + err.Error()), nil
		}
	} else {
		raw, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return jira.ErrorResult(fmt.Sprintf("base64_content is not valid base64: %v", err)), nil
		}
	}
	if len(raw) > jira.MaxAttachmentBytes {
		return jira.ErrorResult(fmt.Sprintf("attachment exceeds the %d MB upload limit", jira.MaxAttachmentBytes>>20)), nil
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return jira.ErrorResult(fmt.Sprintf("failed to build upload: %v", err)), nil
	}
	if _, err := fw.Write(raw); err != nil {
		return jira.ErrorResult(fmt.Sprintf("failed to build upload: %v", err)), nil
	}
	if err := w.Close(); err != nil {
		return jira.ErrorResult(fmt.Sprintf("failed to build upload: %v", err)), nil
	}

	resp, err := jira.DoRaw(auth, http.MethodPost, "/issue/"+key+"/attachments", w.FormDataContentType(), &buf)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	if err := jira.CheckResponse(resp); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	// The attachments endpoint returns a JSON array of attachment objects.
	var arr []map[string]interface{}
	if err := json.Unmarshal(resp.Body, &arr); err != nil {
		return jira.ErrorResult(fmt.Sprintf("failed to parse Jira response: %v", err)), nil
	}
	if len(arr) == 0 {
		return jira.ErrorResult("Jira accepted the upload but returned no attachment"), nil
	}

	return jira.ResourceResult(arr[0], fmt.Sprintf("Uploaded attachment %s to %s", fileName, key)), nil
}
