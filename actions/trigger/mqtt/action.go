package trigger_mqtt

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "MQTT Trigger"
	Description  = "Start a flow whenever a message arrives on an MQTT topic. Flomation holds a subscription open to your broker and runs the flow the moment a message is published. Wildcards are supported: + matches one level, # matches the rest."
	Website      = "https://www.flomation.co"
	Icon         = "tower-broadcast"
	Date         = "12/07/2026"
	Type         = core.ActionTypeTrigger
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
	// Some brokers pin the client ID in their access rules — AWS IoT Core and
	// Azure IoT Hub both require the device's own name — and refuse the connection
	// otherwise. Left empty, a stable ID is derived from the trigger.
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Leave empty unless your broker requires a specific client ID (AWS IoT Core, Azure IoT Hub)"},
	{Name: "ws_path", Type: core.ConnectionTypeString, Label: "WebSocket Path", Placeholder: "/mqtt", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"ws", "wss"}}},
	{Name: "ca_certificate", Type: core.ConnectionTypeSecret, Label: "CA Certificate", Placeholder: "PEM-encoded CA cert — only for a privately-signed broker", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "client_certificate", Type: core.ConnectionTypeSecret, Label: "Client Certificate", Placeholder: "PEM-encoded client cert — only for mutual-TLS brokers", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "client_key", Type: core.ConnectionTypeSecret, Label: "Client Key", Placeholder: "PEM-encoded client private key — only for mutual-TLS brokers", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-signed broker", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},

	{Name: "topics", Type: core.ConnectionTypeString, Label: "Topics", Placeholder: "sensors/# — separate several with commas; add :1 or :2 after a topic to raise its delivery guarantee", Required: true},
	// Named default_qos, not qos: the inbound message carries its own qos onto
	// this node's outputs, and an input of the same name would be clobbered by it.
	{Name: "default_qos", Type: core.ConnectionTypeString, Label: "Delivery Guarantee (QoS)", Placeholder: "Applies to every topic that doesn't set its own", Options: []core.ConnectionOption{
		{Name: "0 — At most once (fire and forget)", Value: "0"},
		{Name: "1 — At least once (acknowledged)", Value: "1"},
		{Name: "2 — Exactly once", Value: "2"},
	}},
	{Name: "parse_json", Type: core.ConnectionTypeBoolean, Label: "Parse JSON Payload", Placeholder: "Decode the payload into fields you can reference downstream"},
	{Name: "durable", Type: core.ConnectionTypeBoolean, Label: "Queue Messages While Disconnected", Placeholder: "Ticked (default): the broker holds QoS 1 and 2 messages for this flow if the connection drops, and delivers them on reconnect"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Message Summary"},
	{Name: "topic", Type: core.ConnectionTypeString, Label: "Topic"},
	{Name: "payload", Type: core.ConnectionTypeString, Label: "Payload"},
	{Name: "payload_json", Type: core.ConnectionTypeObject, Label: "Payload (JSON)"},
	{Name: "qos", Type: core.ConnectionTypeInteger, Label: "QoS"},
	{Name: "retained", Type: core.ConnectionTypeBoolean, Label: "Was Retained"},
	{Name: "received_at", Type: core.ConnectionTypeString, Label: "Received At"},
	{Name: "broker", Type: core.ConnectionTypeString, Label: "Broker"},
}

// configInputs are the trigger's own settings — the broker credentials and the
// subscription config. They must never be echoed onto the output ports, both
// because the password would leak downstream and because they would collide with
// the inbound message fields of the same name (topic, qos).
var configInputs = map[string]bool{
	"protocol":           true,
	"host":               true,
	"port":               true,
	"username":           true,
	"password":           true,
	"client_id":          true,
	"ws_path":            true,
	"ca_certificate":     true,
	"client_certificate": true,
	"client_key":         true,
	"allow_insecure":     true,
	"topics":             true,
	"default_qos":        true,
	"parse_json":         true,
	"durable":            true,
	"__node_id":          true,
}

// Execute runs once the message has already arrived — launch holds the
// subscription and injects the message onto this node's inputs, so all this does
// is marshal them onto the outputs.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing MQTT trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	result["content"] = buildContentSummary(result)

	return result, nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func buildContentSummary(data map[string]interface{}) string {
	topic := str(data["topic"])
	payload := str(data["payload"])

	if topic == "" {
		topic = "an MQTT topic"
	}
	if payload == "" {
		return fmt.Sprintf("[MQTT] %s", topic)
	}
	if len(payload) > 200 {
		payload = payload[:200] + "…"
	}
	return fmt.Sprintf("[MQTT] %s — %s", topic, payload)
}
