package sleep

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Sleep"
	Description  = "Pause execution for a specified duration"
	Website      = "https://www.flomation.co"
	Icon         = "clock"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction

	maxDurationSeconds = 3600 // 1 hour safety cap
)

var Inputs = [...]core.Connection{
	{
		Name:        "duration",
		Type:        core.ConnectionTypeInteger,
		Label:       "Duration",
		Placeholder: "10",
		Required:    true,
	},
	{
		Name:  "unit",
		Type:  core.ConnectionTypeString,
		Label: "Unit",
		Options: []core.ConnectionOption{
			{Name: "Seconds", Value: "seconds"},
			{Name: "Minutes", Value: "minutes"},
			{Name: "Hours", Value: "hours"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "slept_for_seconds", Type: core.ConnectionTypeInteger, Label: "Slept For (seconds)"},
	{Name: "cancelled", Type: core.ConnectionTypeBoolean, Label: "Was Cancelled"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	durationConn := core.FindConnection("duration", inputs)
	if durationConn == nil || durationConn.Number() == nil {
		return nil, fmt.Errorf("duration is required")
	}
	durationVal := *durationConn.Number()

	unitConn := core.FindConnection("unit", inputs)
	unit := "seconds"
	if unitConn != nil && unitConn.String() != nil && *unitConn.String() != "" {
		unit = *unitConn.String()
	}

	var totalSeconds int64
	switch unit {
	case "minutes":
		totalSeconds = durationVal * 60
	case "hours":
		totalSeconds = durationVal * 3600
	default:
		totalSeconds = durationVal
	}

	if totalSeconds <= 0 {
		return map[string]interface{}{
			"slept_for_seconds": int64(0),
			"cancelled":         false,
		}, nil
	}

	if totalSeconds > maxDurationSeconds {
		totalSeconds = maxDurationSeconds
	}

	ctx := flow.GoContext()
	timer := time.NewTimer(time.Duration(totalSeconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		return map[string]interface{}{
			"slept_for_seconds": totalSeconds,
			"cancelled":         false,
		}, nil
	case <-ctx.Done():
		return map[string]interface{}{
			"slept_for_seconds": int64(0),
			"cancelled":         true,
		}, nil
	}
}
