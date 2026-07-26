package crm_salesforce_event_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Event"
	Description  = "Move, shorten or amend an appointment already in Salesforce — a reschedule, a change of room, a different attendee. Anything left blank stays as it is."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pencil"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID", Placeholder: "00U5f00000AbCdEEAV", Required: true},
	{Name: "event_subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Discovery call"},
	{Name: "start_date_time", Type: core.ConnectionTypeDateTime, Label: "Starts", Placeholder: "The new start time"},
	{Name: "end_date_time", Type: core.ConnectionTypeDateTime, Label: "Ends", Placeholder: "The new finish time"},
	{Name: "duration_minutes", Type: core.ConnectionTypeInteger, Label: "Length (minutes)", Placeholder: "30 — an alternative to a finish time"},
	{Name: "is_all_day_event", Type: core.ConnectionTypeBoolean, Label: "All-Day Event"},
	{Name: "activity_date", Type: core.ConnectionTypeDateTime, Label: "Event Date (all-day)", Placeholder: "The day an all-day event falls on"},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location", Placeholder: "Head office, Teams, or a customer address"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Replaces whatever is on the event now"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Whose Calendar", Placeholder: "Salesforce user ID to move the event to"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "Contact or Lead", Placeholder: "Record ID of the person being met"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case"},
	{Name: "show_as", Type: core.ConnectionTypeString, Label: "Show Time As", Placeholder: "Busy, Free or OutOfOffice"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private"},
	{Name: "is_reminder_set", Type: core.ConnectionTypeBoolean, Label: "Set a Reminder"},
	{Name: "reminder_date_time", Type: core.ConnectionTypeDateTime, Label: "Remind At", Placeholder: "When Salesforce should nudge the owner"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Event"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	eventID := salesforce.OptionalString("event_id", inputs)
	if err := salesforce.ValidateRecordID(eventID); err != nil {
		return nil, err
	}

	// Only the boxes the operator actually filled in are sent. Salesforce treats
	// an omitted field as "leave it alone" and an explicit null as "clear it", so
	// an update that posted every blank input would strip the meeting bare.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Subject", "event_subject")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Location", "location")
	salesforce.SetIfPresent(body, inputs, "StartDateTime", "start_date_time")
	salesforce.SetIfPresent(body, inputs, "EndDateTime", "end_date_time")
	salesforce.SetIntIfPresent(body, inputs, "DurationInMinutes", "duration_minutes")
	salesforce.SetBoolIfSet(body, inputs, "IsAllDayEvent", "is_all_day_event")
	// ActivityDate is the all-day event's Date field — a Date, not a DateTime —
	// so the picker's value is trimmed to the day. A full timestamp is rejected.
	salesforce.SetDateIfPresent(body, inputs, "ActivityDate", "activity_date")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "WhoId", "who_id")
	salesforce.SetIfPresent(body, inputs, "WhatId", "what_id")
	salesforce.SetIfPresent(body, inputs, "ShowAs", "show_as")
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")
	salesforce.SetIfPresent(body, inputs, "ReminderDateTime", "reminder_date_time")
	salesforce.SetBoolIfSet(body, inputs, "IsReminderSet", "is_reminder_set")

	// A reminder time on its own never fires — Salesforce needs the flag too — so
	// rescheduling a reminder switches it on unless it was deliberately unticked.
	if _, hasReminder := body["ReminderDateTime"]; hasReminder {
		if _, flagged := body["IsReminderSet"]; !flagged {
			body["IsReminderSet"] = true
		}
	}

	// EventSubtype and the recurrence pattern are deliberately absent: Salesforce
	// fixes both when the event is created. Changing a repeating series is done
	// by cancelling it from an occurrence onwards (Cancel Event Series) and
	// booking a new one, not by patching the original.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the event")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Event", eventID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no record to
	// return, only the ID we already hold, which is what the next node chains off.
	summary := fmt.Sprintf("Updated event %s (%s)", eventID, strings.Join(salesforce.SortedKeys(body), ", "))
	return salesforce.RecordResult(eventID, nil, summary), nil
}
