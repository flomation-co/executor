package qr

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "QR Code Trigger"
	Description  = "Triggers a flow when a QR code is scanned"
	Website      = "https://www.flomation.co"
	Icon         = "qrcode"
	Date         = "23/03/2026"
	Type         = core.ActionTypeTrigger
)

var Outputs = [...]core.Connection{
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
	{Name: "ip", Type: core.ConnectionTypeString, Label: "Client IP"},
	{Name: "user_agent", Type: core.ConnectionTypeString, Label: "User Agent"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing QR code trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
