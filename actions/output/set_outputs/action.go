package output_set_outputs

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Set Outputs"
	Description  = "Set all parent outputs as flow outputs"
	Website      = "https://www.flomation.co"
	Icon         = "list-check"
	Date         = "25/03/2026"
	Type         = core.ActionTypeOutput
)

var Inputs = [...]core.Connection{}

var Outputs = [...]core.Connection{}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	count := 0

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
			flow.SetOutput(k, v)
			count++
		}
	}

	return map[string]interface{}{
		"count": int64(count),
	}, nil
}
