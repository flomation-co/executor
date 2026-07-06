package jira_attachment_get

import (
	"encoding/base64"
	"fmt"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Attachment"
	Description  = "Fetch a Jira attachment's details by its ID. Optionally download the file itself — when Download File is on, the file's bytes are returned base64-encoded on the File Content output."
	Website      = "https://www.flomation.co"
	Icon         = "jira+paperclip"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment ID", Placeholder: "The attachment's numeric ID, e.g. 10001", Required: true},
	{Name: "download", Type: core.ConnectionTypeBoolean, Label: "Download File", Placeholder: "Also download the file's bytes (returned base64-encoded)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Attachment"},
	{Name: "base64_content", Type: core.ConnectionTypeString, Label: "File Content (base64)"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := jira.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := jira.RequiredString("attachment_id", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	obj, err := jira.GetResource(auth, "/attachment/"+id, nil)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	fileName, _ := obj["filename"].(string)
	summary := "Fetched attachment " + id
	if fileName != "" {
		summary = "Fetched attachment " + fileName
	}
	out := jira.ResourceResult(obj, summary)

	if jira.OptionalBool("download", inputs) {
		contentURL, _ := obj["content"].(string)
		if contentURL == "" {
			return jira.ErrorResult("attachment has no downloadable content URL"), nil
		}
		data, _, err := jira.GetBinary(auth, contentURL)
		if err != nil {
			return jira.ErrorResult(err.Error()), nil
		}
		out["base64_content"] = base64.StdEncoding.EncodeToString(data)
		out["tool_result"] = fmt.Sprintf("%s (%d bytes downloaded)", summary, len(data))
	}

	return out, nil
}
