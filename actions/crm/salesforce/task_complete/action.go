package crm_salesforce_task_complete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Mark Task Complete"
	Description  = "Tick a Salesforce task off. No need to know which status value your org counts as done — it uses Completed unless you tell it otherwise."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// completedStatus is Salesforce's out-of-the-box "done" value for Task.Status.
// It is the default rather than something the operator has to look up, because
// knowing that "Completed" is the magic word is exactly the sort of CRM trivia
// this action exists to remove. An org that renamed the value can override it.
const completedStatus = "Completed"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "00T5f00000AbCdEEAV", Required: true},
	{Name: "task_status", Type: core.ConnectionTypeString, Label: "Completed Status", Placeholder: "Completed — only change this if your org renamed it"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Comments", Placeholder: "Optional — replaces the comments on the task, e.g. how the call went"},
	{Name: "call_disposition", Type: core.ConnectionTypeString, Label: "Call Result", Placeholder: "Optional — e.g. Spoke to customer"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Task"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	taskID := salesforce.OptionalString("task_id", inputs)
	if err := salesforce.ValidateRecordID(taskID); err != nil {
		return nil, err
	}

	status := salesforce.OptionalString("task_status", inputs)
	if status == "" {
		status = completedStatus
	}

	// Salesforce fills in CompletedDateTime itself the moment the status becomes
	// a completed one — it is read-only, so there is deliberately nothing here
	// that tries to set it.
	body := map[string]interface{}{"Status": status}
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "CallDisposition", "call_disposition")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Task", taskID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// The update answers 204 No Content, so the ID we were handed is the only
	// thing to pass on — and it is what the next step needs.
	return salesforce.RecordResult(taskID, nil, fmt.Sprintf("Marked task %s as %s", taskID, status)), nil
}
