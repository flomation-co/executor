package messagebrokers_mqtt_retained_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	mqtt "flomation.app/automate/executor/actions/messagebrokers/mqtt"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "MQTT: Get Retained Value"
	Description  = "Read a topic's last known value — the message the broker has retained. Use it to ask 'what is the current temperature?' without waiting for the next reading. Check the Found output to branch on whether the topic has a stored value."
	Website      = "https://www.flomation.co"
	Icon         = "tower-broadcast+magnifying-glass"
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

	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic", Placeholder: "sensors/kitchen/temperature — the topic whose stored value you want", Required: true},
	{Name: "parse_json", Type: core.ConnectionTypeBoolean, Label: "Parse JSON Payload", Placeholder: "Decode the payload into fields you can reference downstream"},
}

var Outputs = [...]core.Connection{
	{Name: "found", Type: core.ConnectionTypeBoolean, Label: "Found"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "payload", Type: core.ConnectionTypeString, Label: "Value"},
	{Name: "payload_json", Type: core.ConnectionTypeObject, Label: "Value (JSON)"},
	{Name: "qos", Type: core.ConnectionTypeInteger, Label: "QoS"},
	{Name: "retained", Type: core.ConnectionTypeBoolean, Label: "Was Retained"},
	{Name: "received_at", Type: core.ConnectionTypeString, Label: "Read At"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Message"},
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
	parseJSON := mqtt.OptionalBool("parse_json", inputs)

	client, err := mqtt.Connect(auth, "get")
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	defer mqtt.Disconnect(client)

	// The broker delivers a retained message immediately on subscribe or never,
	// so a short window is enough — and only the retained flag distinguishes the
	// stored value from live traffic that happens to arrive while we are listening.
	msg, err := mqtt.AwaitMessage(flow, client, topic, 1, mqtt.RetainedWait, func(m mqtt.Message) bool {
		return m.Retained
	})
	if err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}

	if msg == nil {
		return map[string]interface{}{
			"found":       false,
			"topic":       topic,
			"tool_result": fmt.Sprintf("%s has no retained value", topic),
			"success":     true,
		}, nil
	}

	out := mqtt.MessageResult(msg.Topic, msg.Payload, msg.QoS, msg.Retained, parseJSON)
	out["found"] = true
	out["result"] = map[string]interface{}{
		"topic":    msg.Topic,
		"payload":  msg.Payload,
		"qos":      int64(msg.QoS),
		"retained": msg.Retained,
	}
	out["tool_result"] = fmt.Sprintf("%s = %s", msg.Topic, mqtt.Preview(msg.Payload))
	out["success"] = true

	return out, nil
}
