package conditional_for

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "For"
	Description  = "Loop a fixed number of times"
	Website      = "https://www.flomation.co"
	Icon         = "arrow-rotate-right"
	Date         = "25/03/2026"
	Type         = core.ActionTypeLoop
)

var Inputs = [...]core.Connection{
	{
		Name:        "count",
		Type:        core.ConnectionTypeInteger,
		Label:       "Iterations",
		Placeholder: "10",
		Required:    true,
	},
	{
		Name:        "start",
		Type:        core.ConnectionTypeInteger,
		Label:       "Start Index",
		Placeholder: "0",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "result",
		Type:  core.ConnectionTypeBoolean,
		Label: "Continue",
	},
	{
		Name:  "iterations",
		Type:  core.ConnectionTypeInteger,
		Label: "Iteration Count",
	},
	{
		Name:  "max_iterations",
		Type:  core.ConnectionTypeInteger,
		Label: "Total Iterations",
	},
	{
		Name:  "current_index",
		Type:  core.ConnectionTypeInteger,
		Label: "Current Index",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	countConn := core.FindConnection("count", inputs)
	if countConn == nil || countConn.String() == nil {
		return nil, fmt.Errorf("iteration count is required")
	}

	count, err := strconv.ParseInt(*countConn.String(), 10, 64)
	if err != nil || count < 0 {
		return nil, fmt.Errorf("invalid iteration count: %v", countConn.Value)
	}

	start := int64(0)
	startConn := core.FindConnection("start", inputs)
	if startConn != nil && startConn.String() != nil {
		if v, err := strconv.ParseInt(*startConn.String(), 10, 64); err == nil {
			start = v
		}
	}

	// Check current iteration from loop context variable
	currentIndex := start
	shouldContinue := count > 0
	if loopIdx, ok := flow.GetVariable("loop.index"); ok {
		if idx, ok := loopIdx.(int64); ok {
			currentIndex = start + idx
			shouldContinue = idx < count
		}
	}

	return map[string]interface{}{
		"result":         shouldContinue,
		"iterations":     count,
		"max_iterations": count,
		"current_index":  currentIndex,
	}, nil
}
