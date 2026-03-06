package manual

import (
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Manual Trigger"
	Description  = "A simple trigger invoked by pressing the play button, or as the default means"
	Website      = "https://www.flomation.co"
	Icon         = "fa-solid fa-play"
	Date         = "27/11/2025"
)

var Outputs = [...]core.Connection{
	core.Connection{
		Name:        "start",
		Type:        core.ConnectionTypeString,
		Label:       "Start Time",
		Placeholder: "",
	},
	core.Connection{
		Name:        "quote",
		Type:        core.ConnectionTypeString,
		Label:       "Quote",
		Placeholder: "",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing manual trigger")

	return map[string]interface{}{
		"start": time.Now().UTC().Format(time.RFC1123),
		"quote": "To err is human",
	}, nil
}
