package crm_salesforce_task_update

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Task"
	Description  = "Change an existing Salesforce task — hand it to someone else, move the due date, raise the priority or record how a call went. Anything left blank is left exactly as it is."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pencil"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "00T5f00000AbCdEEAV", Required: true},
	{Name: "task_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "In Progress"},
	{Name: "task_subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Call back about the quote"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Comments", Placeholder: "Replaces whatever is on the task now"},
	{Name: "activity_date", Type: core.ConnectionTypeDateTime, Label: "Due Date", Placeholder: "The day it is due — any time of day is ignored"},
	{Name: "task_priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "High"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assigned To", Placeholder: "Salesforce user ID of the new owner"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "Contact or Lead", Placeholder: "Record ID of the person this is about"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case"},
	{Name: "task_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Call, Email, Meeting or Other"},
	{Name: "call_type", Type: core.ConnectionTypeString, Label: "Call Direction", Placeholder: "Inbound, Outbound or Internal"},
	{Name: "call_disposition", Type: core.ConnectionTypeString, Label: "Call Result", Placeholder: "Left voicemail"},
	{Name: "call_object", Type: core.ConnectionTypeString, Label: "Call Recording ID", Placeholder: "The call's ID in your phone system"},
	{Name: "call_duration_seconds", Type: core.ConnectionTypeInteger, Label: "Call Length (seconds)", Placeholder: "180"},
	{Name: "is_reminder_set", Type: core.ConnectionTypeBoolean, Label: "Set a Reminder"},
	{Name: "reminder_date_time", Type: core.ConnectionTypeDateTime, Label: "Remind At", Placeholder: "When Salesforce should nudge the owner"},
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

	taskID := salesforce.OptionalString("task_id", inputs)
	if err := salesforce.ValidateRecordID(taskID); err != nil {
		return nil, err
	}

	// Only the boxes the operator actually filled in are sent. Salesforce treats
	// an omitted field as "leave it alone" and an explicit null as "clear it", so
	// an update that posted every blank input would wipe half the task.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Status", "task_status")
	salesforce.SetIfPresent(body, inputs, "Subject", "task_subject")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Priority", "task_priority")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "WhoId", "who_id")
	salesforce.SetIfPresent(body, inputs, "WhatId", "what_id")
	// TaskSubtype is deliberately absent: Salesforce fixes it at creation and
	// rejects any attempt to change it, so offering it here would only produce a
	// confusing failure. Type is the editable picklist and is offered instead.
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

	// A reminder time on its own never fires — Salesforce needs the flag too — so
	// rescheduling a reminder switches it on unless it was deliberately unticked.
	if _, hasTime := body["ReminderDateTime"]; hasTime {
		if _, flagged := body["IsReminderSet"]; !flagged {
			body["IsReminderSet"] = true
		}
	}

	salesforce.SetIfPresent(body, inputs, "RecurrenceType", "recurrence_type")
	salesforce.SetIfPresent(body, inputs, "RecurrenceInstance", "recurrence_instance")
	salesforce.SetIntIfPresent(body, inputs, "RecurrenceInterval", "recurrence_interval")
	salesforce.SetIntIfPresent(body, inputs, "RecurrenceDayOfMonth", "recurrence_day_of_month")
	salesforce.SetIfPresent(body, inputs, "RecurrenceMonthOfYear", "recurrence_month_of_year")
	salesforce.SetIfPresent(body, inputs, "RecurrenceRegeneratedType", "recurrence_regenerated_type")
	salesforce.SetIfPresent(body, inputs, "RecurrenceTimeZoneSidKey", "recurrence_timezone")
	// Each end of the repeat window gets its own key. n8n declares its start-date
	// input under the end date's name, so changing the start there silently moves
	// the END of the series instead.
	salesforce.SetDateIfPresent(body, inputs, "RecurrenceStartDateOnly", "recurrence_start_date")
	salesforce.SetDateIfPresent(body, inputs, "RecurrenceEndDateOnly", "recurrence_end_date")

	mask, err := recurrenceDayMask(salesforce.OptionalString("recurrence_days_of_week", inputs))
	if err != nil {
		return nil, err
	}
	if mask > 0 {
		body["RecurrenceDayOfWeekMask"] = mask
	}

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the task")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Task", taskID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no record to
	// return, only the ID we already hold, which is what the next node chains off.
	summary := fmt.Sprintf("Updated task %s (%s)", taskID, strings.Join(salesforce.SortedKeys(body), ", "))
	return salesforce.RecordResult(taskID, nil, summary), nil
}

// dayOfWeekBits maps a weekday onto its bit in Salesforce's
// RecurrenceDayOfWeekMask. Salesforce stores the chosen weekdays as ONE summed
// number — Sunday 1, Monday 2, Tuesday 4, up to Saturday 64 — so "Tuesday and
// Thursday" is 20. That is arithmetic no front-of-house operator should be asked
// to do, which is why this action takes day names and n8n takes the raw number.
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
// number passes straight through so anyone following Salesforce's own docs can
// still paste 20 and get 20. Bits are OR-ed rather than summed so a duplicated
// day cannot corrupt the value.
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
