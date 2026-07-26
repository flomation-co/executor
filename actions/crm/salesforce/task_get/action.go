package crm_salesforce_task_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Task"
	Description  = "Look up one Salesforce task by its record ID and return everything on it, so a later step can check who it is assigned to, when it is due or whether it is done."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+eye"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "00T5f00000AbCdEEAV", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Subject, Status, ActivityDate — leave blank for every field"},
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

	// Check the field list here as well as inside the shared client, so a typo
	// is reported as the configuration mistake it is rather than landing on the
	// error port as though Salesforce had refused the call.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, err
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Task", taskID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Fetched task %s", taskID)
	if subject, ok := record["Subject"].(string); ok && subject != "" {
		summary = fmt.Sprintf("Fetched task %q (%s)", subject, taskID)
	}
	return salesforce.RecordResult(taskID, record, summary), nil
}
