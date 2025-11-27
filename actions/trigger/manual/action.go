package manual

import (
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing manual trigger")
	return map[string]interface{}{
		"start": time.Now().UTC(),
		"quote": "To err is human",
	}, nil
}
