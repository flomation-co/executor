package messagebrokers_mqtt_request_reply

import (
	"fmt"

	core "flomation.app/automate/executor"
	mqtt "flomation.app/automate/executor/actions/messagebrokers/mqtt"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "MQTT: Publish and Await Reply"
	Description  = "Publish a request to one topic and wait for the answer on a reply topic — ask a device a question and get its response in a single step. Check the Replied output to branch on whether an answer came back."
	Website      = "https://www.flomation.co"
	Icon         = "tower-broadcast+reply"
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

	{Name: "topic", Type: core.ConnectionTypeString, Label: "Request Topic", Placeholder: "devices/boiler/command — where to send the request", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Request Message", Placeholder: "The payload to send — plain text or JSON"},
	{Name: "reply_topic", Type: core.ConnectionTypeString, Label: "Reply Topic", Placeholder: "devices/boiler/status — where the answer will arrive (wildcards allowed)", Required: true},
	{Name: "qos", Type: core.ConnectionTypeString, Label: "Delivery Guarantee (QoS)", Options: []core.ConnectionOption{
		{Name: "0 — At most once (fire and forget)", Value: "0"},
		{Name: "1 — At least once (acknowledged)", Value: "1"},
		{Name: "2 — Exactly once", Value: "2"},
	}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Wait For Reply (seconds)", Placeholder: "30 — how long to wait for the answer before giving up (maximum 300)"},
	{Name: "parse_json", Type: core.ConnectionTypeBoolean, Label: "Parse JSON Reply", Placeholder: "Decode the reply into fields you can reference downstream"},
}

var Outputs = [...]core.Connection{
	{Name: "replied", Type: core.ConnectionTypeBoolean, Label: "Replied"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Reply Topic"},
	{Name: "payload", Type: core.ConnectionTypeString, Label: "Reply"},
	{Name: "payload_json", Type: core.ConnectionTypeObject, Label: "Reply (JSON)"},
	{Name: "qos", Type: core.ConnectionTypeInteger, Label: "QoS"},
	{Name: "retained", Type: core.ConnectionTypeBoolean, Label: "Was Retained"},
	{Name: "received_at", Type: core.ConnectionTypeString, Label: "Replied At"},
	{Name: "request_topic", Type: core.ConnectionTypeString, Label: "Request Topic"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Reply Message"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := mqtt.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	requestTopic, err := mqtt.RequiredString("topic", "Request Topic", inputs)
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	if err := mqtt.ValidatePublishTopic(requestTopic); err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	replyTopic, err := mqtt.RequiredString("reply_topic", "Reply Topic", inputs)
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}

	message := mqtt.OptionalString("message", inputs)
	qos := mqtt.ParseQoS("qos", inputs)
	wait := mqtt.WaitSeconds("timeout_seconds", inputs, mqtt.DefaultWaitSeconds)
	parseJSON := mqtt.OptionalBool("parse_json", inputs)

	client, err := mqtt.Connect(auth, "req")
	if err != nil {
		return mqtt.ErrorResult(err.Error()), nil
	}
	defer mqtt.Disconnect(client)

	// Subscribe before publishing. A device can answer in single-digit
	// milliseconds, so subscribing afterwards would race the reply and lose it.
	listener, err := mqtt.Listen(client, replyTopic, qos)
	if err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}
	defer listener.Close(client)

	if err := mqtt.Publish(client, requestTopic, qos, false, message); err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}

	// A retained value on the reply topic is a stale answer to somebody else's
	// question, not ours — never accept it as the reply.
	msg, err := listener.Await(flow, wait, func(m mqtt.Message) bool { return !m.Retained })
	if err != nil {
		return mqtt.ErrorResult(mqtt.Redact(auth, err.Error())), nil
	}

	if msg == nil {
		return map[string]interface{}{
			"replied":       false,
			"request_topic": requestTopic,
			"topic":         replyTopic,
			"tool_result":   fmt.Sprintf("Sent the request to %s but no reply arrived on %s within %s", requestTopic, replyTopic, wait),
			"success":       true,
		}, nil
	}

	out := mqtt.MessageResult(msg.Topic, msg.Payload, msg.QoS, msg.Retained, parseJSON)
	out["replied"] = true
	out["request_topic"] = requestTopic
	out["result"] = map[string]interface{}{
		"topic":    msg.Topic,
		"payload":  msg.Payload,
		"qos":      int64(msg.QoS),
		"retained": msg.Retained,
	}
	out["tool_result"] = fmt.Sprintf("Replied on %s: %s", msg.Topic, mqtt.Preview(msg.Payload))
	out["success"] = true

	return out, nil
}
