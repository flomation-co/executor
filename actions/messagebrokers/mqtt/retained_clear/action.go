package messagebrokers_mqtt_retained_clear

import (
	"fmt"

	core "flomation.app/automate/executor"
	mqtt "flomation.app/automate/executor/actions/messagebrokers/mqtt"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "MQTT: Clear Retained Value"
	Description  = "Delete a topic's stored last known value, so new subscribers no longer receive a stale reading. Live messages on the topic are unaffected."
	Website      = "https://www.flomation.co"
	Icon         = "tower-broadcast+trash"
	Date         = "12/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: []core.ConnectionOption{
		{Name: "MQTT (TCP)", Value: "mqtt"},
		{Name: "MQTT over TLS", Value: "mqtts"},
		{Name: "MQTT over WebSocket", Value: "ws"},
		{Name: "MQTT over Secure WebSocket", Value: "wss"},
	}},
	{Name: "host", Type: core.ConnectionTypeString, Label: "Broker Host", Placeholder: "broker.example.com — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "1883 (MQTT), 8883 (TLS), 8083 (WebSocket)"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Leave empty for an anonymous broker"},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Broker password"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Leave empty to generate a unique ID per run (recommended)"},
	{Name: "ws_path", Type: core.ConnectionTypeString, Label: "WebSocket Path", Placeholder: "/mqtt", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"ws", "wss"}}},
	{Name: "ca_certificate", Type: core.ConnectionTypeSecret, Label: "CA Certificate", Placeholder: "PEM-encoded CA cert — only for a privately-signed broker", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "client_certificate", Type: core.ConnectionTypeSecret, Label: "Client Certificate", Placeholder: "PEM-encoded client cert — only for mutual-TLS brokers", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "client_key", Type: core.ConnectionTypeSecret, Label: "Client Key", Placeholder: "PEM-encoded client private key — only for mutual-TLS brokers", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-signed broker", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},

	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic", Placeholder: "sensors/kitchen/temperature — the topic whose stored value to clear", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := mqtt.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	topic, err := mqtt.RequiredString("topic", "Topic", inputs)
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	if err := mqtt.ValidatePublishTopic(topic); err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}

	client, err := mqtt.Connect(auth, "clr")
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	defer mqtt.Disconnect(client)

	// The protocol defines "clear the retained value" as publishing a zero-length
	// retained payload. QoS 1 so the broker acknowledges the deletion rather than
	// letting it race the disconnect.
	if err := mqtt.Publish(client, topic, 1, true, ""); err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}

	return map[string]interface{}{
		"topic":       topic,
		"result":      map[string]interface{}{"topic": topic, "cleared": true},
		"tool_result": fmt.Sprintf("Cleared the retained value on %s", topic),
		"success":     true,
	}, nil
}
