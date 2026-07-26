package crm_salesforce_event_create

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
	Name         = "Salesforce: Create Event"
	Description  = "Put a meeting, site visit or call in the diary in Salesforce and link it to the person and the deal it is about — the step that turns an online booking into an appointment on the rep's calendar."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+calendar"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "event_subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Discovery call"},
	{Name: "start_date_time", Type: core.ConnectionTypeDateTime, Label: "Starts", Placeholder: "When the appointment starts"},
	{Name: "end_date_time", Type: core.ConnectionTypeDateTime, Label: "Ends", Placeholder: "When it finishes — or give a Length instead"},
	{Name: "duration_minutes", Type: core.ConnectionTypeInteger, Label: "Length (minutes)", Placeholder: "30 — used when there is no finish time"},
	{Name: "is_all_day_event", Type: core.ConnectionTypeBoolean, Label: "All-Day Event"},
	{Name: "activity_date", Type: core.ConnectionTypeDateTime, Label: "Event Date (all-day)", Placeholder: "The day an all-day event falls on"},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location", Placeholder: "Head office, Teams, or a customer address"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Agenda, joining details, anything the attendee needs"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Whose Calendar", Placeholder: "Salesforce user ID of the person the event belongs to"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "Contact or Lead", Placeholder: "Record ID of the person you are meeting, e.g. 0035f00000AbCdEAAV"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case this is about"},
	{Name: "show_as", Type: core.ConnectionTypeString, Label: "Show Time As", Placeholder: "Busy, Free or OutOfOffice"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private"},
	{Name: "event_subtype", Type: core.ConnectionTypeString, Label: "Event Kind", Placeholder: "Event — cannot be changed afterwards"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Record type ID, if your org uses them for events"},
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
	{Name: "recurrence_start_date_time", Type: core.ConnectionTypeDateTime, Label: "Repeat From", Placeholder: "First occurrence of the repeating series"},
	{Name: "recurrence_end_date", Type: core.ConnectionTypeDateTime, Label: "Repeat Until", Placeholder: "Last day of the repeating series"},
	{Name: "recurrence_timezone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London"},
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
	// WhoId is the person being met (a Contact or Lead) and WhatId is what the
	// meeting is about (account, opportunity, case). Salesforce enforces which
	// object types are allowed in each and the shared client translates its
	// complaint, so the IDs go through as given.
	salesforce.SetIfPresent(body, inputs, "WhoId", "who_id")
	salesforce.SetIfPresent(body, inputs, "WhatId", "what_id")
	salesforce.SetIfPresent(body, inputs, "ShowAs", "show_as")
	salesforce.SetBoolIfSet(body, inputs, "IsPrivate", "is_private")
	// EventSubtype is create-only in Salesforce, which is precisely why it is
	// offered here and not on the update action.
	salesforce.SetIfPresent(body, inputs, "EventSubtype", "event_subtype")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	salesforce.SetIfPresent(body, inputs, "ReminderDateTime", "reminder_date_time")
	salesforce.SetBoolIfSet(body, inputs, "IsReminderSet", "is_reminder_set")

	// A reminder needs BOTH the flag and the time. Salesforce accepts either on
	// its own and then quietly does nothing, so a reminder time is taken as
	// intent and switches the flag on unless it was deliberately turned off.
	if _, hasReminder := body["ReminderDateTime"]; hasReminder {
		if _, flagged := body["IsReminderSet"]; !flagged {
			body["IsReminderSet"] = true
		}
	} else if on, _ := body["IsReminderSet"].(bool); on {
		log.Warn("Salesforce create event: Set a Reminder is ticked but no Remind At time was given — Salesforce accepts this but the reminder never fires")
	}

	salesforce.SetIfPresent(body, inputs, "RecurrenceType", "recurrence_type")
	salesforce.SetIfPresent(body, inputs, "RecurrenceInstance", "recurrence_instance")
	salesforce.SetIntIfPresent(body, inputs, "RecurrenceInterval", "recurrence_interval")
	salesforce.SetIntIfPresent(body, inputs, "RecurrenceDayOfMonth", "recurrence_day_of_month")
	salesforce.SetIfPresent(body, inputs, "RecurrenceMonthOfYear", "recurrence_month_of_year")
	salesforce.SetIfPresent(body, inputs, "RecurrenceTimeZoneSidKey", "recurrence_timezone")
	// An event's series starts at a date AND time (unlike a task, whose repeat
	// window is two plain dates) but still ends on a date only.
	salesforce.SetIfPresent(body, inputs, "RecurrenceStartDateTime", "recurrence_start_date_time")
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
	// an event created as a one-off can never be turned into a series later.
	// Anyone who filled in a repeat pattern meant it.
	if _, flagged := body["IsRecurrence"]; !flagged {
		if _, repeating := body["RecurrenceType"]; repeating {
			body["IsRecurrence"] = true
		}
	}

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	// Salesforce's own rule, checked here so the operator gets a sentence rather
	// than REQUIRED_FIELD_MISSING: an event needs a start, and unless it is an
	// all-day event it needs either a finish time or a length. The checks run on
	// the assembled body so a start supplied through Additional Fields (as
	// ActivityDateTime, the older name for the same thing) counts too.
	_, hasStart := body["StartDateTime"]
	if _, ok := body["ActivityDateTime"]; ok {
		hasStart = true
	}
	_, hasDate := body["ActivityDate"]
	_, hasEnd := body["EndDateTime"]
	_, hasDuration := body["DurationInMinutes"]
	if allDay, _ := body["IsAllDayEvent"].(bool); allDay {
		if !hasDate && !hasStart {
			return nil, fmt.Errorf("an all-day event needs a date — set Event Date (all-day)")
		}
	} else {
		if !hasStart {
			return nil, fmt.Errorf("an event needs a start time — set Starts, or tick All-Day Event and set Event Date (all-day)")
		}
		if !hasEnd && !hasDuration {
			return nil, fmt.Errorf("an event needs an end — set Ends, or a Length in minutes")
		}
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Event", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Created event %s", id)
	if subject := salesforce.OptionalString("event_subject", inputs); subject != "" {
		summary = fmt.Sprintf("Created event %q (%s)", subject, id)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// dayOfWeekBits maps a weekday onto its bit in Salesforce's
// RecurrenceDayOfWeekMask. Salesforce stores the chosen weekdays as ONE summed
// number — Sunday 1, Monday 2, Tuesday 4, up to Saturday 64 — so "Tuesday and
// Thursday" is 20. That is arithmetic no front-of-house operator should be asked
// to do, which is why this action takes day names.
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
