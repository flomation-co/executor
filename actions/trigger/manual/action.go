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
	Icon         = "play"
	Date         = "27/11/2025"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing manual trigger")

	return map[string]interface{}{
		"start": time.Now().UTC(),
		"quote": "To err is human",
	}, nil
}
