package jira_issue_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Issue"
	Description  = "Create a Jira issue. Pick the project and issue type, set a summary and description, and optionally assign it, set a priority, add labels or link a parent (for subtasks). Any other field can be set via Custom Fields or Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "jira+plus"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "Choose a project", Required: true},
	{Name: "issue_type", Type: core.ConnectionTypeString, Label: "Issue Type", Placeholder: "Choose an issue type", Required: true},
	{Name: "summary", Type: core.ConnectionTypeString, Label: "Summary", Placeholder: "A short one-line title for the issue", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "The issue description (plain text)"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "Choose a priority"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "Choose a user to assign"},
	{Name: "reporter", Type: core.ConnectionTypeString, Label: "Reporter", Placeholder: "Choose the reporting user (defaults to you)"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated labels, e.g. backend,urgent"},
	{Name: "components", Type: core.ConnectionTypeString, Label: "Component IDs", Placeholder: "Comma-separated component IDs"},
	{Name: "parent_key", Type: core.ConnectionTypeString, Label: "Parent Issue Key", Placeholder: "Required for a subtask — the parent issue key, e.g. SCRUM-12"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due Date", Placeholder: "YYYY-MM-DD"},
	{Name: "custom_fields", Type: core.ConnectionTypeObject, Label: "Custom Fields (JSON)", Placeholder: `{"customfield_10011":"value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"environment":"production"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Issue Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Issue"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := jira.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	summary, err := jira.RequiredString("summary", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	projectID, err := jira.RequiredString("project", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	issueTypeID, err := jira.RequiredString("issue_type", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	fields := map[string]interface{}{
		"summary":   summary,
		"project":   map[string]interface{}{"id": projectID},
		"issuetype": map[string]interface{}{"id": issueTypeID},
	}
	jira.SetIfPresent(fields, inputs, "description", "description")
	jira.SetIfPresent(fields, inputs, "duedate", "due_date")
	if v := jira.OptionalString("priority", inputs); v != "" {
		fields["priority"] = map[string]interface{}{"id": v}
	}
	// Cloud identifies users by accountId (the GDPR-era documented property); the
	// live dropdown emits an accountId as its value.
	if v := jira.OptionalString("assignee", inputs); v != "" {
		fields["assignee"] = map[string]interface{}{"accountId": v}
	}
	if v := jira.OptionalString("reporter", inputs); v != "" {
		fields["reporter"] = map[string]interface{}{"accountId": v}
	}
	if labels := jira.StringList("labels", inputs); len(labels) > 0 {
		fields["labels"] = labels
	}
	if comps := jira.StringList("components", inputs); len(comps) > 0 {
		out := make([]interface{}, 0, len(comps))
		for _, c := range comps {
			out = append(out, map[string]interface{}{"id": c})
		}
		fields["components"] = out
	}
	if v := jira.OptionalString("parent_key", inputs); v != "" {
		fields["parent"] = map[string]interface{}{"key": v}
	}
	// custom_fields is a flat object of customfield_x:value pairs merged directly
	// onto fields (each key IS the target field id), so merge it explicitly.
	if raw, err := jira.OptionalJSON("custom_fields", inputs); err != nil {
		return jira.ErrorResult(err.Error()), nil
	} else if obj, ok := raw.(map[string]interface{}); ok {
		for k, val := range obj {
			fields[k] = val
		}
	}
	if err := jira.MergeAdditionalFields(fields, inputs); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	resp, err := jira.CreateResource(auth, "/issue", map[string]interface{}{"fields": fields})
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	out := jira.ResourceResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Created issue %s", out["id"])
	return out, nil
}
