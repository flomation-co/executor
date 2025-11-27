package output

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing output")

	name := core.FindConnection("output", inputs)
	value := core.FindConnection("count", inputs)

	flow.SetOutput(name.String(), value.Value)

	return nil, nil
}
