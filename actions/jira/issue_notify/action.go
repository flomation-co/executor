package jira_issue_notify

import (
	core "flomation.app/automate/executor"
	jira "flomation.app/automate/executor/actions/jira"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Notify About Issue"
	Description  = "Send an email notification about a Jira issue. Set a subject and message body, and choose who receives it — the reporter, assignee, watchers, voters, or named users and groups."
	Website      = "https://www.flomation.co"
	Icon         = "jira+bell"
	Date         = "06/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-domain.atlassian.net", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Account Email", Placeholder: "The Atlassian account email that owns the API token", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens", Required: true},
	{Name: "issue_key", Type: core.ConnectionTypeString, Label: "Issue Key", Placeholder: "The issue key, e.g. SCRUM-1", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "The email subject line"},
	{Name: "text_body", Type: core.ConnectionTypeText, Label: "Text Body", Placeholder: "The plain-text message body"},
	{Name: "html_body", Type: core.ConnectionTypeText, Label: "HTML Body", Placeholder: "The HTML message body"},
	{Name: "notify_reporter", Type: core.ConnectionTypeBoolean, Label: "Notify Reporter", Placeholder: "Email the issue reporter"},
	{Name: "notify_assignee", Type: core.ConnectionTypeBoolean, Label: "Notify Assignee", Placeholder: "Email the issue assignee"},
	{Name: "notify_watchers", Type: core.ConnectionTypeBoolean, Label: "Notify Watchers", Placeholder: "Email everyone watching the issue"},
	{Name: "notify_voters", Type: core.ConnectionTypeBoolean, Label: "Notify Voters", Placeholder: "Email everyone who voted on the issue"},
	{Name: "to_users", Type: core.ConnectionTypeString, Label: "To Users", Placeholder: "Comma-separated account IDs to notify"},
	{Name: "to_groups", Type: core.ConnectionTypeString, Label: "To Groups", Placeholder: "Comma-separated group names to notify"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Issue Key"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	body := map[string]interface{}{}
	jira.SetIfPresent(body, inputs, "subject", "subject")
	jira.SetIfPresent(body, inputs, "textBody", "text_body")
	jira.SetIfPresent(body, inputs, "htmlBody", "html_body")

	to := map[string]interface{}{}
	jira.SetBoolIfSet(to, inputs, "reporter", "notify_reporter")
	jira.SetBoolIfSet(to, inputs, "assignee", "notify_assignee")
	jira.SetBoolIfSet(to, inputs, "watchers", "notify_watchers")
	jira.SetBoolIfSet(to, inputs, "voters", "notify_voters")
	if users := jira.StringList("to_users", inputs); len(users) > 0 {
		out := make([]interface{}, 0, len(users))
		for _, id := range users {
			out = append(out, map[string]interface{}{"accountId": id})
		}
		to["users"] = out
	}
	if groups := jira.StringList("to_groups", inputs); len(groups) > 0 {
		out := make([]interface{}, 0, len(groups))
		for _, name := range groups {
			out = append(out, map[string]interface{}{"name": name})
		}
		to["groups"] = out
	}
	if len(to) > 0 {
		body["to"] = to
	}

	if err := jira.PostNoContent(auth, "/issue/"+key+"/notify", body); err != nil {
		return jira.ErrorResult(err.Error()), nil
	}
	return jira.SuccessResult(key, map[string]interface{}{}, "Notification queued for "+key), nil
}
