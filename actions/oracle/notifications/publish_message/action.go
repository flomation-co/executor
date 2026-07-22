// Package oracle_notifications_publish_message publishes a message to a topic — it fans out
// to every confirmed subscription (email, SMS, HTTPS, Slack, …).
package oracle_notifications_publish_message

import (
	"strings"

	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Publish Message"
	Description  = "Publish a message to a Notifications topic — it fans out to every confirmed subscription (email, SMS, HTTPS, Slack and so on). The title becomes the email subject. Choose RAW_TEXT (the body verbatim) or JSON (per-protocol payloads)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bell"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the topic picker)"},
	{Name: "topic_ocid", Type: core.ConnectionTypeString, Label: "Topic OCID", Placeholder: "ocid1.onstopic.oc1..aaaa… to publish to", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "The message title — becomes the email subject (optional but recommended)"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body", Placeholder: "The message body", Required: true},
	{Name: "message_type", Type: core.ConnectionTypeString, Label: "Message Type", Placeholder: `RAW_TEXT (default) sends the body verbatim; JSON needs a body like {"DEFAULT":"…","EMAIL":"…"} — a "DEFAULT" key is required, plus optional per-protocol keys`, Options: []core.ConnectionOption{
		{Name: "Raw text", Value: "RAW_TEXT"},
		{Name: "JSON (per-protocol)", Value: "JSON"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Timestamp"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ons.DataPlaneClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	topicID, err := ons.RequiredString("topic_ocid", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	body, err := ons.RequiredString("body", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	msg := onssdk.MessageDetails{Body: &body}
	if title := ons.OptionalString("title", inputs); title != "" {
		msg.Title = &title
	}
	req := onssdk.PublishMessageRequest{TopicId: &topicID, MessageDetails: msg}
	switch strings.ToUpper(strings.TrimSpace(ons.OptionalString("message_type", inputs))) {
	case "JSON":
		req.MessageType = onssdk.PublishMessageMessageTypeJson
	case "", "RAW_TEXT":
		req.MessageType = onssdk.PublishMessageMessageTypeRawText
	default:
		return ons.ErrorResult("message type must be RAW_TEXT or JSON"), nil
	}
	resp, err := client.PublishMessage(ons.Context(), req)
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	return ons.Result("Published message to the topic", map[string]interface{}{
		"message_id": ons.Str(resp.MessageId),
		"timestamp":  ons.FormatTime(resp.TimeStamp),
	}), nil
}
