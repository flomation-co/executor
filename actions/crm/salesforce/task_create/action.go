package crm_salesforce_task_create

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Task"
	Description  = "Add a to-do to Salesforce — a call to make, an email to send or a follow-up to chase — and link it to the person and the record it is about. Set it to repeat if it happens on a schedule."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "task_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Not Started", Required: true},
	{Name: "task_subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Call back about the quote"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Comments", Placeholder: "Anything the person doing this needs to know"},
	{Name: "activity_date", Type: core.ConnectionTypeDateTime, Label: "Due Date", Placeholder: "The day it is due — any time of day is ignored"},
	{Name: "task_priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "Normal"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assigned To", Placeholder: "Salesforce user ID of the person responsible"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "Contact or Lead", Placeholder: "Record ID of the person this is about, e.g. 0035f00000AbCdEAAV"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case this relates to"},
	{Name: "task_subtype", Type: core.ConnectionTypeString, Label: "Activity Kind", Placeholder: "Task, Call or Email — cannot be changed afterwards"},
	{Name: "task_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Call, Email, Meeting or Other"},
	{Name: "call_type", Type: core.ConnectionTypeString, Label: "Call Direction", Placeholder: "Inbound, Outbound or Internal"},
	{Name: "call_disposition", Type: core.ConnectionTypeString, Label: "Call Result", Placeholder: "Left voicemail"},
	{Name: "call_object", Type: core.ConnectionTypeString, Label: "Call Recording ID", Placeholder: "The call's ID in your phone system"},
	{Name: "call_duration_seconds", Type: core.ConnectionTypeInteger, Label: "Call Length (seconds)", Placeholder: "180"},
	{Name: "is_reminder_set", Type: core.ConnectionTypeBoolean, Label: "Set a Reminder"},
	{Name: "reminder_date_time", Type: core.ConnectionTypeDateTime, Label: "Remind At", Placeholder: "When Salesforce should nudge the owner"},
	{Name: "is_recurrence", Type: core.ConnectionTypeBoolean, Label: "Repeats"},
	{Name: "recurrence_type", Type: core.ConnectionTypeString, Label: "Repeats How Often", Placeholder: "RecursDaily, RecursWeekly, RecursMonthly or RecursYearly"},
	{Name: "recurrence_interval", Type: core.ConnectionTypeInteger, Label: "Repeat Every", Placeholder: "2 with RecursWeekly means every other week"},
	{Name: "recurrence_days_of_week", Type: core.ConnectionTypeString, Label: "Repeat On Days", Placeholder: "Monday, Wednesday, Friday"},
	{Name: "recurrence_day_of_month", Type: core.ConnectionTypeInteger, Label: "Repeat On Day of Month", Placeholder: "15"},
	{Name: "recurrence_instance", Type: core.ConnectionTypeString, Label: "Repeat On Which Week", Placeholder: "First, Second, Third, Fourth or Last"},
	{
		Name:        "recurrence_month_of_year",
		Type:        core.ConnectionTypeString,
		Label:       "Repeat In Month",
		Placeholder: "Only used by a yearly repeat",
		Options: []core.ConnectionOption{
			{Name: "January", Value: "January"},
			{Name: "February", Value: "February"},
			{Name: "March", Value: "March"},
			{Name: "April", Value: "April"},
			{Name: "May", Value: "May"},
			{Name: "June", Value: "June"},
			{Name: "July", Value: "July"},
			{Name: "August", Value: "August"},
			{Name: "September", Value: "September"},
			{Name: "October", Value: "October"},
			{Name: "November", Value: "November"},
			{Name: "December", Value: "December"},
		},
	},
	{Name: "recurrence_start_date", Type: core.ConnectionTypeDateTime, Label: "Repeat From", Placeholder: "First day of the repeating series"},
	{Name: "recurrence_end_date", Type: core.ConnectionTypeDateTime, Label: "Repeat Until", Placeholder: "Last day of the repeating series"},
	{Name: "recurrence_regenerated_type", Type: core.ConnectionTypeString, Label: "Regenerate After", Placeholder: "RecurrenceRegenerateAfterDueDate"},
	{Name: "recurrence_timezone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London"},
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

	status := salesforce.OptionalString("task_status", inputs)
	if status == "" {
		return nil, fmt.Errorf("task_status is required — the task's status, e.g. Not Started, In Progress or Completed")
	}

	body := map[string]interface{}{"Status": status}
	salesforce.SetIfPresent(body, inputs, "Subject", "task_subject")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Priority", "task_priority")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	// WhoId must be a Contact or Lead and WhatId must be anything but a person
	// (account, opportunity, case). Salesforce enforces that itself and answers
	// MALFORMED_ID or INVALID_CROSS_REFERENCE_KEY, which the shared client
	// already translates, so they are passed through as the operator gave them.
	salesforce.SetIfPresent(body, inputs, "WhoId", "who_id")
	salesforce.SetIfPresent(body, inputs, "WhatId", "what_id")
	// TaskSubtype (Task / Call / Email) is create-only in Salesforce — it cannot
	// be changed afterwards — while Type is the ordinary editable picklist. n8n
	// exposes only TaskSubtype and labels it "Type", so both appear here under
	// honest names and the update action carries Type alone.
	salesforce.SetIfPresent(body, inputs, "TaskSubtype", "task_subtype")
	salesforce.SetIfPresent(body, inputs, "Type", "task_type")
	salesforce.SetIfPresent(body, inputs, "CallType", "call_type")
	salesforce.SetIfPresent(body, inputs, "CallDisposition", "call_disposition")
	salesforce.SetIfPresent(body, inputs, "CallObject", "call_object")
	salesforce.SetIntIfPresent(body, inputs, "CallDurationInSeconds", "call_duration_seconds")
	// ActivityDate is a Date field, not a DateTime: Salesforce rejects a full
	// timestamp outright, so the date picker's value is trimmed to YYYY-MM-DD.
	salesforce.SetDateIfPresent(body, inputs, "ActivityDate", "activity_date")
	salesforce.SetIfPresent(body, inputs, "ReminderDateTime", "reminder_date_time")
	salesforce.SetBoolIfSet(body, inputs, "IsReminderSet", "is_reminder_set")

	// A reminder needs BOTH the flag and the time. Salesforce accepts either on
	// its own and then quietly does nothing, so setting a reminder time is taken
	// as intent and switches the flag on unless it was deliberately turned off.
	if _, hasTime := body["ReminderDateTime"]; hasTime {
		if _, flagged := body["IsReminderSet"]; !flagged {
			body["IsReminderSet"] = true
		}
	} else if on, _ := body["IsReminderSet"].(bool); on {
		log.Warn("Salesforce create task: Set a Reminder is ticked but no Remind At time was given — Salesforce accepts this but the reminder never fires")
	}

	salesforce.SetIfPresent(body, inputs, "RecurrenceType", "recurrence_type")
	salesforce.SetIfPresent(body, inputs, "RecurrenceInstance", "recurrence_instance")
	salesforce.SetIntIfPresent(body, inputs, "RecurrenceInterval", "recurrence_interval")
	salesforce.SetIntIfPresent(body, inputs, "RecurrenceDayOfMonth", "recurrence_day_of_month")
	salesforce.SetIfPresent(body, inputs, "RecurrenceMonthOfYear", "recurrence_month_of_year")
	salesforce.SetIfPresent(body, inputs, "RecurrenceRegeneratedType", "recurrence_regenerated_type")
	salesforce.SetIfPresent(body, inputs, "RecurrenceTimeZoneSidKey", "recurrence_timezone")
	// The repeat window is two Date-only fields. n8n declares its "Recurrence
	// Start Date Only" input under the END date's name, so choosing it silently
	// overwrites the end date and the start date can never be set at all — each
	// gets its own key here.
	salesforce.SetDateIfPresent(body, inputs, "RecurrenceStartDateOnly", "recurrence_start_date")
	salesforce.SetDateIfPresent(body, inputs, "RecurrenceEndDateOnly", "recurrence_end_date")

	mask, err := recurrenceDayMask(salesforce.OptionalString("recurrence_days_of_week", inputs))
	if err != nil {
		return nil, err
	}
	if mask > 0 {
		body["RecurrenceDayOfWeekMask"] = mask
	}

	salesforce.SetBoolIfSet(body, inputs, "IsRecurrence", "is_recurrence")
	// Salesforce ignores every recurrence field unless IsRecurrence is true, and
	// the flag cannot be switched on later — a task created as a one-off stays a
	// one-off. Anyone who filled in a repeat pattern meant it, so the flag
	// follows rather than leaving them with a silently non-repeating task.
	if _, flagged := body["IsRecurrence"]; !flagged {
		if _, repeating := body["RecurrenceType"]; repeating {
			body["IsRecurrence"] = true
		}
	}

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Task", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Created task %s", id)
	if subject := salesforce.OptionalString("task_subject", inputs); subject != "" {
		summary = fmt.Sprintf("Created task %q (%s)", subject, id)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// dayOfWeekBits maps a weekday onto its bit in Salesforce's
// RecurrenceDayOfWeekMask. Salesforce stores the chosen weekdays as ONE summed
// number — Sunday 1, Monday 2, Tuesday 4, up to Saturday 64 — so "Tuesday and
// Thursday" is 20. That is arithmetic no front-of-house operator should be asked
// to do, which is why this action takes day names and n8n takes the raw number.
// Short forms are accepted because upstream systems rarely agree on spelling.
var dayOfWeekBits = map[string]int{
	"sunday": 1, "sun": 1, "su": 1,
	"monday": 2, "mon": 2, "mo": 2,
	"tuesday": 4, "tue": 4, "tues": 4, "tu": 4,
	"wednesday": 8, "wed": 8, "we": 8,
	"thursday": 16, "thu": 16, "thur": 16, "thurs": 16, "th": 16,
	"friday": 32, "fri": 32, "fr": 32,
	"saturday": 64, "sat": 64, "sa": 64,
}

// recurrenceDayMask turns "Monday, Wednesday" into Salesforce's bitmask. A bare
// number passes straight through so anyone following Salesforce's own docs (or
// migrating a flow from another tool) can still paste 20 and get 20. Bits are
// OR-ed rather than summed so a duplicated day cannot corrupt the value.
func recurrenceDayMask(raw string) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n, nil
	}
	mask := 0
	for _, part := range salesforce.SplitList(v) {
		bit, ok := dayOfWeekBits[strings.ToLower(part)]
		if !ok {
			return 0, fmt.Errorf("%q is not a day of the week — use names like Monday, Wednesday, Friday", part)
		}
		mask |= bit
	}
	return mask, nil
}
