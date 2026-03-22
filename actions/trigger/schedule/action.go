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
		},
	},
	{
		Name:        "interval",
		Type:        core.ConnectionTypeString,
		Label:       "Interval",
		Placeholder: "e.g. 15",
	},
	{
		Name:  "unit",
		Type:  core.ConnectionTypeString,
		Label: "Unit",
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
	},
	{
		Name:        "days_of_week",
		Type:        core.ConnectionTypeString,
		Label:       "Days of Week",
		Placeholder: "monday,wednesday,friday",
	},
	{
		Name:        "timezone",
		Type:        core.ConnectionTypeString,
		Label:       "Timezone",
		Placeholder: "Europe/London",
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
