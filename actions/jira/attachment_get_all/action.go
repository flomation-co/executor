package jira_attachment_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Many Attachments"
	Description  = "List every attachment on a Jira issue. Provide the issue key; the attachments are read from the issue itself. Turn on Return All to get them all, or set a Limit to cap how many are returned."
	Website      = "https://www.flomation.co"
	Icon         = "jira+paperclip"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue to read attachments from, e.g. SCRUM-1", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every attachment on the issue"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number to return when Return All is off"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Attachments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
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

	// Jira has no list-attachments endpoint — read them off the issue itself.
	obj, err := jira.GetResource(auth, "/issue/"+key, url.Values{"fields": []string{"attachment"}})
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	f, _ := obj["fields"].(map[string]interface{})
	atts, _ := f["attachment"].([]interface{})
	total := len(atts)

	returnAll := jira.OptionalBool("return_all", inputs)
	if !returnAll {
		if limit, ok := jira.OptionalInt("limit", inputs); ok && limit > 0 && len(atts) > limit {
			atts = atts[:limit]
		}
	}

	return jira.ListResult(atts, total, fmt.Sprintf("Found %d attachment(s) on %s", len(atts), key)), nil
}
