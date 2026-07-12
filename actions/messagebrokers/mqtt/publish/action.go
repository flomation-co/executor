package messagebrokers_mqtt_publish

import (
	"fmt"

	core "flomation.app/automate/executor"
	mqtt "flomation.app/automate/executor/actions/messagebrokers/mqtt"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "MQTT: Publish Message"
	Description  = "Publish a message to a topic on an MQTT broker. Choose the delivery guarantee (QoS) and optionally retain the message so the next subscriber receives it immediately."
	Website      = "https://www.flomation.co"
	Icon         = "tower-broadcast+paper-plane"
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

	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic", Placeholder: "sensors/kitchen/temperature — wildcards are not allowed when publishing", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "The payload to publish — plain text or JSON"},
	{Name: "qos", Type: core.ConnectionTypeString, Label: "Delivery Guarantee (QoS)", Options: []core.ConnectionOption{
		{Name: "0 — At most once (fire and forget)", Value: "0"},
		{Name: "1 — At least once (acknowledged)", Value: "1"},
		{Name: "2 — Exactly once", Value: "2"},
	}},
	{Name: "retain", Type: core.ConnectionTypeBoolean, Label: "Retain", Placeholder: "Keep this as the topic's last known value, delivered to every new subscriber"},
}

var Outputs = [...]core.Connection{
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Published Message"},
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
	// Wildcards are a subscribe-side concept; a broker rejects them on publish
	// with an unhelpful protocol error, so name the mistake here instead.
	if err := mqtt.ValidatePublishTopic(topic); err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}

	message := mqtt.OptionalString("message", inputs)
	qos := mqtt.ParseQoS("qos", inputs)
	retain := mqtt.OptionalBool("retain", inputs)

	client, err := mqtt.Connect(auth, "pub")
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	defer mqtt.Disconnect(client)

	if err := mqtt.Publish(client, topic, qos, retain, message); err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}

	result := map[string]interface{}{
		"topic":    topic,
		"payload":  message,
		"qos":      int64(qos),
		"retained": retain,
		"bytes":    len(message),
	}

	summary := fmt.Sprintf("Published %d bytes to %s at QoS %d", len(message), topic, qos)
	if retain {
		summary += " (retained)"
	}

	return map[string]interface{}{
		"topic":       topic,
		"result":      result,
		"tool_result": summary,
		"success":     true,
	}, nil
}
