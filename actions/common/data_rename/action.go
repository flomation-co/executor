package data_rename

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Data Rename"
	Description  = "Rename an input key to a different output key"
	Website      = "https://www.flomation.co"
	Icon         = "i-cursor"
	Date         = "21/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "input_key",
		Type:        core.ConnectionTypeString,
		Label:       "Input Key",
		Placeholder: "The key to read from parent outputs",
		Required:    true,
	},
	{
		Name:        "output_key",
		Type:        core.ConnectionTypeString,
		Label:       "Output Key",
		Placeholder: "The new key name for the value",
		Required:    true,
	},
	{
		Name:        "value",
		Type:        core.ConnectionTypeString,
		Label:       "Value (optional — if set, uses this instead of looking up input_key from parents)",
		Placeholder: "${text}",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name:  "renamed",
		Type:  core.ConnectionTypeObject,
		Label: "Renamed Data",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	inputKeyConn := core.FindConnection("input_key", inputs)
	inputKey := inputKeyConn.String()
	if inputKey == nil || strings.TrimSpace(*inputKey) == "" {
		return nil, fmt.Errorf("input_key is required")
	}

	outputKeyConn := core.FindConnection("output_key", inputs)
	outputKey := outputKeyConn.String()
	if outputKey == nil || strings.TrimSpace(*outputKey) == "" {
		return nil, fmt.Errorf("output_key is required")
	}

	trimmedInputKey := strings.TrimSpace(*inputKey)
	trimmedOutputKey := strings.TrimSpace(*outputKey)

	// If an explicit value is provided (e.g. ${text}), use it directly.
	// This is the preferred approach — it works regardless of wiring
	// because normal variable substitution resolves the reference.
	valueConn := core.FindConnection("value", inputs)
	if valueConn != nil && valueConn.Value != nil {
		if s, ok := valueConn.Value.(string); ok && s != "" {
			return map[string]interface{}{
				trimmedOutputKey: s,
			}, nil
		}
	}

	// Fall back to looking up input_key by name from parent outputs.
	var value interface{}
	found := false
	parents := flow.FindSource(node.ID)
	for _, parent := range parents {
		if parent == nil {
			continue
		}

		result := flow.GetNodeResult(parent.ID)
		if result == nil {
			continue
		}

		if v, exists := result[trimmedInputKey]; exists {
			value = v
			found = true
			break
		}
	}

	if !found {
		log.WithFields(log.Fields{
			"input_key": trimmedInputKey,
		}).Warn("Input key not found in parent outputs")
	}

	return map[string]interface{}{
		trimmedOutputKey: value,
	}, nil
}
