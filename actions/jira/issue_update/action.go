package jira_issue_update

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Issue"
	Description  = "Update an existing Jira issue. Change any of the standard fields, and optionally move the issue to a new status by choosing a transition. Leave a field blank to leave it unchanged."
	Website      = "https://www.flomation.co"
	Icon         = "jira+pen"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue key to update, e.g. SCRUM-1", Required: true},
	{Name: "summary", Type: core.ConnectionTypeString, Label: "Summary", Placeholder: "A short one-line title for the issue"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "The issue description (plain text)"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "Choose a priority"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "Choose a user to assign"},
	{Name: "reporter", Type: core.ConnectionTypeString, Label: "Reporter", Placeholder: "Choose the reporting user"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated labels, e.g. backend,urgent"},
	{Name: "components", Type: core.ConnectionTypeString, Label: "Component IDs", Placeholder: "Comma-separated component IDs"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due Date", Placeholder: "YYYY-MM-DD"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status Transition", Placeholder: "Choose a transition to move the issue's status"},
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
	key, err := jira.RequiredString("issue_key", inputs)
	if err != nil {
		return jira.ErrorResult(err.Error()), nil
	}

	fields := map[string]interface{}{}
	jira.SetIfPresent(fields, inputs, "summary", "summary")
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

	statusID := jira.OptionalString("status", inputs)
	if statusID == "" && len(fields) == 0 {
		return jira.ErrorResult("nothing to update"), nil
	}

	// A status change is a transition, applied first so the issue is in the target
	// state before the field edits land.
	if statusID != "" {
		body := map[string]interface{}{
			"transition": map[string]interface{}{"id": statusID},
		}
		if err := jira.PostNoContent(auth, "/issue/"+key+"/transitions", body); err != nil {
			return jira.ErrorResult(err.Error()), nil
		}
	}
	if len(fields) > 0 {
		if err := jira.UpdateResource(auth, "/issue/"+key, map[string]interface{}{"fields": fields}); err != nil {
			return jira.ErrorResult(err.Error()), nil
		}
	}

	return jira.SuccessResult(key, map[string]interface{}{}, "Updated issue "+key), nil
}
