package schedule

import (
	core "flomation.app/automate/executor"

	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Schedule Trigger"
	Description  = "Triggers a flow on a recurring schedule"
	Website      = "https://www.flomation.co"
	Icon         = "clock"
	Date         = "22/03/2026"
	Type         = core.ActionTypeTrigger
)

// Modes that fire at a wall-clock time of day all share the time_of_day,
// timezone and exclude_bank_holidays inputs. NOTE: the manifest generator's
// parseVisibleWhen only resolves an inline []string composite literal — a
// package-level variable reference is silently skipped and would emit empty
// Values, so each VisibleWhen below must inline the full mode list.
var Inputs = [...]core.Connection{
	{
		Name:     "mode",
		Type:     core.ConnectionTypeString,
		Label:    "Schedule Mode",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Every interval", Value: "interval"},
			{Name: "Daily at a specific time", Value: "daily"},
			{Name: "Weekly on specific days", Value: "weekly"},
			{Name: "Monthly on specific dates", Value: "monthly"},
			{Name: "Monthly on a weekday", Value: "monthly_weekday"},
			{Name: "Yearly on a date", Value: "yearly"},
		},
	},
	{
		Name:        "interval",
		Type:        core.ConnectionTypeString,
		Label:       "Interval",
		Placeholder: "e.g. 15",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "mode", Values: []string{"interval"}},
	},
	{
		Name:    "unit",
		Type:    core.ConnectionTypeString,
		Label:   "Unit",
		Visible: &core.VisibleWhen{Field: "mode", Values: []string{"interval"}},
		Options: []core.ConnectionOption{
			{Name: "Minutes", Value: "minutes"},
			{Name: "Hours", Value: "hours"},
			{Name: "Days", Value: "days"},
		},
	},
	{
		Name:        "time_of_day",
		Type:        core.ConnectionTypeString,
		Label:       "Time of Day",
		Placeholder: "HH:MM (24-hour)",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "mode", Values: []string{"daily", "weekly", "monthly", "monthly_weekday", "yearly"}},
	},
	{
		Name:     "days_of_week",
		Type:     core.ConnectionTypeMultiSelect,
		Label:    "Days of Week",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "mode", Values: []string{"weekly"}},
		Options: []core.ConnectionOption{
			{Name: "Monday", Value: "monday"},
			{Name: "Tuesday", Value: "tuesday"},
			{Name: "Wednesday", Value: "wednesday"},
			{Name: "Thursday", Value: "thursday"},
			{Name: "Friday", Value: "friday"},
			{Name: "Saturday", Value: "saturday"},
			{Name: "Sunday", Value: "sunday"},
		},
	},
	{
		Name:     "days_of_month",
		Type:     core.ConnectionTypeMultiSelect,
		Label:    "Dates of Month",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "mode", Values: []string{"monthly"}},
		Options: []core.ConnectionOption{
			{Name: "1", Value: "1"},
			{Name: "2", Value: "2"},
			{Name: "3", Value: "3"},
			{Name: "4", Value: "4"},
			{Name: "5", Value: "5"},
			{Name: "6", Value: "6"},
			{Name: "7", Value: "7"},
			{Name: "8", Value: "8"},
			{Name: "9", Value: "9"},
			{Name: "10", Value: "10"},
			{Name: "11", Value: "11"},
			{Name: "12", Value: "12"},
			{Name: "13", Value: "13"},
			{Name: "14", Value: "14"},
			{Name: "15", Value: "15"},
			{Name: "16", Value: "16"},
			{Name: "17", Value: "17"},
			{Name: "18", Value: "18"},
			{Name: "19", Value: "19"},
			{Name: "20", Value: "20"},
			{Name: "21", Value: "21"},
			{Name: "22", Value: "22"},
			{Name: "23", Value: "23"},
			{Name: "24", Value: "24"},
			{Name: "25", Value: "25"},
			{Name: "26", Value: "26"},
			{Name: "27", Value: "27"},
			{Name: "28", Value: "28"},
			{Name: "29", Value: "29"},
			{Name: "30", Value: "30"},
			{Name: "31", Value: "31"},
			{Name: "Last day of month", Value: "last"},
		},
	},
	{
		Name:     "week_ordinal",
		Type:     core.ConnectionTypeString,
		Label:    "Which Week",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "mode", Values: []string{"monthly_weekday"}},
		Options: []core.ConnectionOption{
			{Name: "First", Value: "first"},
			{Name: "Second", Value: "second"},
			{Name: "Third", Value: "third"},
			{Name: "Fourth", Value: "fourth"},
			{Name: "Fifth", Value: "fifth"},
			{Name: "Last", Value: "last"},
		},
	},
	{
		Name:     "weekday",
		Type:     core.ConnectionTypeString,
		Label:    "Weekday",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "mode", Values: []string{"monthly_weekday"}},
		Options: []core.ConnectionOption{
			{Name: "Monday", Value: "monday"},
			{Name: "Tuesday", Value: "tuesday"},
			{Name: "Wednesday", Value: "wednesday"},
			{Name: "Thursday", Value: "thursday"},
			{Name: "Friday", Value: "friday"},
			{Name: "Saturday", Value: "saturday"},
			{Name: "Sunday", Value: "sunday"},
		},
	},
	{
		Name:     "month_of_year",
		Type:     core.ConnectionTypeString,
		Label:    "Month",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "mode", Values: []string{"yearly"}},
		Options: []core.ConnectionOption{
			{Name: "January", Value: "1"},
			{Name: "February", Value: "2"},
			{Name: "March", Value: "3"},
			{Name: "April", Value: "4"},
			{Name: "May", Value: "5"},
			{Name: "June", Value: "6"},
			{Name: "July", Value: "7"},
			{Name: "August", Value: "8"},
			{Name: "September", Value: "9"},
			{Name: "October", Value: "10"},
			{Name: "November", Value: "11"},
			{Name: "December", Value: "12"},
		},
	},
	{
		Name:     "day_of_month",
		Type:     core.ConnectionTypeString,
		Label:    "Day of Month",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "mode", Values: []string{"yearly"}},
		Options: []core.ConnectionOption{
			{Name: "1", Value: "1"},
			{Name: "2", Value: "2"},
			{Name: "3", Value: "3"},
			{Name: "4", Value: "4"},
			{Name: "5", Value: "5"},
			{Name: "6", Value: "6"},
			{Name: "7", Value: "7"},
			{Name: "8", Value: "8"},
			{Name: "9", Value: "9"},
			{Name: "10", Value: "10"},
			{Name: "11", Value: "11"},
			{Name: "12", Value: "12"},
			{Name: "13", Value: "13"},
			{Name: "14", Value: "14"},
			{Name: "15", Value: "15"},
			{Name: "16", Value: "16"},
			{Name: "17", Value: "17"},
			{Name: "18", Value: "18"},
			{Name: "19", Value: "19"},
			{Name: "20", Value: "20"},
			{Name: "21", Value: "21"},
			{Name: "22", Value: "22"},
			{Name: "23", Value: "23"},
			{Name: "24", Value: "24"},
			{Name: "25", Value: "25"},
			{Name: "26", Value: "26"},
			{Name: "27", Value: "27"},
			{Name: "28", Value: "28"},
			{Name: "29", Value: "29"},
			{Name: "30", Value: "30"},
			{Name: "31", Value: "31"},
		},
	},
	{
		Name:        "timezone",
		Type:        core.ConnectionTypeString,
		Label:       "Timezone",
		Placeholder: "Europe/London",
		Visible:     &core.VisibleWhen{Field: "mode", Values: []string{"daily", "weekly", "monthly", "monthly_weekday", "yearly"}},
	},
	{
		Name:    "exclude_bank_holidays",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Exclude UK Bank Holidays",
		Visible: &core.VisibleWhen{Field: "mode", Values: []string{"daily", "weekly", "monthly", "monthly_weekday", "yearly"}},
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "triggered_at",
		Type:  core.ConnectionTypeString,
		Label: "Triggered At",
	},
	{
		Name:  "schedule_mode",
		Type:  core.ConnectionTypeString,
		Label: "Schedule Mode",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing schedule trigger")

	result := make(map[string]interface{})

	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
