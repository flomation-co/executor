package task_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Task: Create"
	Description  = "Create Apollo tasks for one or more contacts, assigned to a user."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+list-check"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Assignee (User ID)", Placeholder: "Apollo user ID to assign the task to", Required: true},
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated Apollo contact IDs", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "call", Options: []core.ConnectionOption{
		{Name: "Call", Value: "call"},
		{Name: "Action Item", Value: "action_item"},
		{Name: "Email (Manual)", Value: "email_manual"},
		{Name: "LinkedIn Step: Message", Value: "linkedin_step_message"},
		{Name: "LinkedIn Step: Connect", Value: "linkedin_step_connect"},
		{Name: "LinkedIn Step: View Profile", Value: "linkedin_step_view_profile"},
	}},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "scheduled", Options: []core.ConnectionOption{
		{Name: "Scheduled", Value: "scheduled"},
		{Name: "Completed", Value: "completed"},
		{Name: "Skipped", Value: "skipped"},
	}},
	{Name: "due_at", Type: core.ConnectionTypeDateTime, Label: "Due At", Placeholder: "2026-09-30T09:00:00Z"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "medium", Options: []core.ConnectionOption{
		{Name: "High", Value: "high"},
		{Name: "Medium", Value: "medium"},
		{Name: "Low", Value: "low"},
	}},
	{Name: "note", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "Optional note for the task"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tasks"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	if _, err := apollo_common.RequiredString("user_id", inputs); err != nil {
		return apollo_common.ErrorResult("an assignee user ID is required"), nil
	}
	contactIDs := apollo_common.StringList("contact_ids", inputs)
	if len(contactIDs) == 0 {
		return apollo_common.ErrorResult("at least one contact ID is required"), nil
	}

	body := map[string]interface{}{"contact_ids": contactIDs}
	apollo_common.SetString(body, "user_id", "user_id", inputs)
	apollo_common.SetString(body, "type", "type", inputs)
	apollo_common.SetString(body, "status", "status", inputs)
	apollo_common.SetString(body, "due_at", "due_at", inputs)
	apollo_common.SetString(body, "priority", "priority", inputs)
	apollo_common.SetString(body, "note", "note", inputs)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/tasks/bulk_create", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	tasks := apollo_common.Arr(resp, "tasks")
	if len(tasks) == 0 {
		tasks = apollo_common.Arr(resp, "contacts")
	}
	return apollo_common.ListResult(tasks, fmt.Sprintf("Created %d tasks", len(tasks))), nil
}
