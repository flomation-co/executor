package data_combine

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Data Combine"
	Description  = "Combine all parent outputs into a single object"
	Website      = "https://www.flomation.co"
	Icon         = "object-group"
	Date         = "20/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name:  "combined",
		Type:  core.ConnectionTypeObject,
		Label: "Combined Data",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	merged := make(map[string]interface{})

	parents := flow.FindSource(node.ID)
	for _, parent := range parents {
		if parent == nil {
			continue
		}

		result := flow.GetNodeResult(parent.ID)
		if result == nil {
			log.WithFields(log.Fields{
				"parent_id": parent.ID,
			}).Warn("Parent node has no cached result")
			continue
		}

		for k, v := range result {
			if _, exists := merged[k]; exists {
				log.WithFields(log.Fields{
					"key":       k,
					"parent_id": parent.ID,
				}).Warn("Overwriting existing key during combine")
			}
			merged[k] = v
		}
	}

	return map[string]interface{}{
		"combined": merged,
	}, nil
}
