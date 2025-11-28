package output

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Set Output"
	Description  = "Pass data out from a Flow"
	Website      = "https://www.flomation.co"
	Icon         = "fa-solid fa-dollar-sign"
	Date         = "27/11/2025"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing output")

	name := core.FindConnection("output", inputs)
	value := core.FindConnection("count", inputs)

	flow.SetOutput(name.String(), value.Value)

	return nil, nil
}
