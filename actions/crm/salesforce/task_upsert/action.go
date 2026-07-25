package crm_salesforce_task_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Task"
	Description  = "Create a task, or update the existing one that carries the same reference from your other system. Re-running the flow — or a webhook that fires twice — updates the same follow-up instead of piling up duplicates."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+rotate"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "Booking_Reference__c — an External ID field on Task", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match On Value", Placeholder: "The reference this task has in your other system", Required: true},
	{Name: "task_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Not Started"},
	{Name: "task_subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Call back about the quote"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Comments", Placeholder: "Anything the person doing this needs to know"},
	{Name: "activity_date", Type: core.ConnectionTypeDateTime, Label: "Due Date", Placeholder: "The day it is due — any time of day is ignored"},
	{Name: "task_priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "Normal"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assigned To", Placeholder: "Salesforce user ID of the person responsible"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "Contact or Lead", Placeholder: "Record ID of the person this is about"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case"},
	{Name: "task_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Call, Email, Meeting or Other"},
	{Name: "call_type", Type: core.ConnectionTypeString, Label: "Call Direction", Placeholder: "Inbound, Outbound or Internal"},
	{Name: "call_disposition", Type: core.ConnectionTypeString, Label: "Call Result", Placeholder: "Left voicemail"},
	{Name: "call_duration_seconds", Type: core.ConnectionTypeInteger, Label: "Call Length (seconds)", Placeholder: "180"},
	{Name: "is_reminder_set", Type: core.ConnectionTypeBoolean, Label: "Set a Reminder"},
	{Name: "reminder_date_time", Type: core.ConnectionTypeDateTime, Label: "Remind At", Placeholder: "When Salesforce should nudge the owner"},
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

	externalField := salesforce.OptionalString("external_id_field", inputs)
	if externalField == "" {
		return nil, fmt.Errorf("external_id_field is required — the field on Task that holds your other system's reference, e.g. Booking_Reference__c. It must be marked as an External ID in Salesforce")
	}
	if _, err := salesforce.ValidateSOQLFieldName(externalField); err != nil {
		return nil, err
	}
	externalValue := salesforce.OptionalString("external_id_value", inputs)
	if externalValue == "" {
		return nil, fmt.Errorf("external_id_value is required — it is the reference Salesforce matches on to decide whether to create or update")
	}

	// TaskSubtype is deliberately not offered: Salesforce fixes it at creation,
	// so on the "matched an existing task" half of an upsert it would be rejected
	// and the whole step would fail for no good reason.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Status", "task_status")
	salesforce.SetIfPresent(body, inputs, "Subject", "task_subject")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Priority", "task_priority")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "WhoId", "who_id")
	salesforce.SetIfPresent(body, inputs, "WhatId", "what_id")
	salesforce.SetIfPresent(body, inputs, "Type", "task_type")
	salesforce.SetIfPresent(body, inputs, "CallType", "call_type")
	salesforce.SetIfPresent(body, inputs, "CallDisposition", "call_disposition")
	salesforce.SetIntIfPresent(body, inputs, "CallDurationInSeconds", "call_duration_seconds")
	// ActivityDate is a Date field, not a DateTime: Salesforce rejects a full
	// timestamp outright, so the date picker's value is trimmed to YYYY-MM-DD.
	salesforce.SetDateIfPresent(body, inputs, "ActivityDate", "activity_date")
	salesforce.SetIfPresent(body, inputs, "ReminderDateTime", "reminder_date_time")
	salesforce.SetBoolIfSet(body, inputs, "IsReminderSet", "is_reminder_set")
	if _, hasTime := body["ReminderDateTime"]; hasTime {
		// A reminder time without the flag never fires, so setting one turns the
		// flag on unless it was deliberately unticked.
		if _, flagged := body["IsReminderSet"]; !flagged {
			body["IsReminderSet"] = true
		}
	}

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Task", externalField, externalValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	if id == "" {
		// An upsert that MATCHED an existing task answers 204 No Content, so
		// Salesforce never says which record it hit. Look it up by the same
		// reference rather than handing the flow an empty ID nothing downstream
		// can chain off. Best effort only: a failure here does not fail the
		// upsert, which has already succeeded.
		//
		// Typed: an External ID field is frequently Number-typed (a booking
		// number), and quoting a numeric literal is a hard INVALID_FIELD, so a
		// value-only guess would lose the ID for exactly those orgs.
		soql, qErr := salesforce.BuildQueryTyped(instanceURL, token, "Task", "Id", []salesforce.Condition{{Field: externalField, Operator: "=", Value: externalValue}}, false, "", 1, true)
		if qErr == nil {
			if record, qErr := salesforce.QueryOne(instanceURL, token, soql); qErr == nil && record != nil {
				id = salesforce.StringifyID(record["Id"])
			}
		}
	}

	verb := "Updated existing"
	if created {
		verb = "Created"
	}
	summary := fmt.Sprintf("%s task %s, matched on %s = %s", verb, id, externalField, externalValue)
	return salesforce.RecordResult(id, raw, summary), nil
}
