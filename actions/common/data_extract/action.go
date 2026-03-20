package data_extract

import (
	"strings"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Data Extract"
	Description  = "Selectively extract outputs from parent nodes"
	Website      = "https://www.flomation.co"
	Icon         = "filter"
	Date         = "20/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "keys",
		Type:        core.ConnectionTypeText,
		Label:       "Keys to Extract",
		Placeholder: "Comma-separated output names, e.g. repository_path,status",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "extracted",
		Type:  core.ConnectionTypeObject,
		Label: "Extracted Data",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	keysConn := core.FindConnection("keys", inputs)
	keysStr := keysConn.String()
	if keysStr == nil || strings.TrimSpace(*keysStr) == "" {
		return map[string]interface{}{
			"extracted": map[string]interface{}{},
		}, nil
	}

	requestedKeys := make(map[string]bool)
	for _, k := range strings.Split(*keysStr, ",") {
		trimmed := strings.TrimSpace(k)
		if trimmed != "" {
			requestedKeys[trimmed] = true
		}
	}

	// Gather all parent results
	allResults := make(map[string]interface{})
	parents := flow.FindSource(node.ID)
	for _, parent := range parents {
		if parent == nil {
			continue
		}

		result := flow.GetNodeResult(parent.ID)
		if result == nil {
			continue
		}

		for k, v := range result {
			allResults[k] = v
		}
	}

	// Filter to only requested keys
	extracted := make(map[string]interface{})
	for key := range requestedKeys {
		if val, exists := allResults[key]; exists {
			extracted[key] = val
		} else {
			log.WithFields(log.Fields{
				"key": key,
			}).Warn("Requested key not found in parent outputs")
		}
	}

	return map[string]interface{}{
		"extracted": extracted,
	}, nil
}
