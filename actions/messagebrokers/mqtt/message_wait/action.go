package messagebrokers_mqtt_message_wait

import (
	"fmt"

	core "flomation.app/automate/executor"
	mqtt "flomation.app/automate/executor/actions/messagebrokers/mqtt"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "MQTT: Wait for Message"
	Description  = "Pause the flow and wait for the next message on a topic, up to a timeout. Use it to wait for a device to report back mid-flow. Check the Received output to branch on whether a message actually arrived."
	Website      = "https://www.flomation.co"
	Icon         = "tower-broadcast+hourglass-start"
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

	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic", Placeholder: "sensors/kitchen/+ — wildcards allowed (+ one level, # the rest)", Required: true},
	{Name: "qos", Type: core.ConnectionTypeString, Label: "Delivery Guarantee (QoS)", Options: []core.ConnectionOption{
		{Name: "0 — At most once (fire and forget)", Value: "0"},
		{Name: "1 — At least once (acknowledged)", Value: "1"},
		{Name: "2 — Exactly once", Value: "2"},
	}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Wait For (seconds)", Placeholder: "30 — how long to listen before giving up (maximum 300)"},
	{Name: "ignore_retained", Type: core.ConnectionTypeBoolean, Label: "Ignore Retained Value", Placeholder: "Ticked (default): wait for a genuinely new message, skipping the topic's stored last value"},
	{Name: "parse_json", Type: core.ConnectionTypeBoolean, Label: "Parse JSON Payload", Placeholder: "Decode the payload into fields you can reference downstream"},
}

var Outputs = [...]core.Connection{
	{Name: "received", Type: core.ConnectionTypeBoolean, Label: "Received"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "payload", Type: core.ConnectionTypeString, Label: "Payload"},
	{Name: "payload_json", Type: core.ConnectionTypeObject, Label: "Payload (JSON)"},
	{Name: "qos", Type: core.ConnectionTypeInteger, Label: "QoS"},
	{Name: "retained", Type: core.ConnectionTypeBoolean, Label: "Was Retained"},
	{Name: "received_at", Type: core.ConnectionTypeString, Label: "Received At"},
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

	qos := mqtt.ParseQoS("qos", inputs)
	wait := mqtt.WaitSeconds("timeout_seconds", inputs, mqtt.DefaultWaitSeconds)
	// A retained value is the topic's *stored* last message, delivered the moment
	// we subscribe. Waiting for "the next message" almost always means a new one,
	// so the default skips it.
	ignoreRetained := mqtt.BoolWithDefault("ignore_retained", inputs, true)
	parseJSON := mqtt.OptionalBool("parse_json", inputs)

	client, err := mqtt.Connect(auth, "wait")
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	defer mqtt.Disconnect(client)

	msg, err := mqtt.AwaitMessage(flow, client, topic, qos, wait, func(m mqtt.Message) bool {
		return !(ignoreRetained && m.Retained)
	})
	if err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}

	if msg == nil {
		return map[string]interface{}{
			"received":    false,
			"topic":       topic,
			"tool_result": fmt.Sprintf("No message arrived on %s within %s", topic, wait),
			"success":     true,
		}, nil
	}

	out := mqtt.MessageResult(msg.Topic, msg.Payload, msg.QoS, msg.Retained, parseJSON)
	out["received"] = true
	out["result"] = map[string]interface{}{
		"topic":    msg.Topic,
		"payload":  msg.Payload,
		"qos":      int64(msg.QoS),
		"retained": msg.Retained,
	}
	out["tool_result"] = fmt.Sprintf("Received on %s: %s", msg.Topic, mqtt.Preview(msg.Payload))
	out["success"] = true

	return out, nil
}
