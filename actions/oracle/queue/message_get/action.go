// Package oracle_queue_message_get consumes messages from a queue. The operator supplies the queue
// OCID; this action resolves the queue's own messages endpoint automatically and gets a batch of
// messages, each with a receipt token used to delete it or extend its visibility.
package oracle_queue_message_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Get Messages"
	Description  = "Consume a batch of messages from a queue. Each message includes a receipt token for deleting it or updating its visibility; the queue's endpoint is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "queue_ocid", Type: core.ConnectionTypeString, Label: "Queue OCID", Placeholder: "ocid1.queue.oc1..aaaa…", Required: true},
	{Name: "visibility_seconds", Type: core.ConnectionTypeString, Label: "Visibility Timeout (seconds)", Placeholder: "How long the got messages stay hidden; 0 = peek (optional)"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeString, Label: "Long-poll Timeout (seconds)", Placeholder: "Wait up to this long for a message; 0 = short-poll (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Most messages to return in one batch (optional)"},
	{Name: "channel_filter", Type: core.ConnectionTypeString, Label: "Channel Filter", Placeholder: "Only messages on channels matching this prefix (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Messages"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := q.DataPlaneClientForQueue(inputs, queueID)
	if errResult != nil {
		return errResult, nil
	}

	req := queue.GetMessagesRequest{QueueId: &queueID}
	for field, set := range map[string]func(int){
		"visibility_seconds": func(n int) { req.VisibilityInSeconds = &n },
		"timeout_seconds":    func(n int) { req.TimeoutInSeconds = &n },
		"limit":              func(n int) { req.Limit = &n },
	} {
		if n, ok, err := q.OptionalInt(field, inputs); err != nil {
			return q.ErrorResult(err.Error()), nil
		} else if ok {
			set(n)
		}
	}
	if channel := q.OptionalString("channel_filter", inputs); channel != "" {
		req.ChannelFilter = &channel
	}

	resp, err := client.GetMessages(q.Context(), req)
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	out := make([]map[string]interface{}, 0, len(resp.Messages))
	for i := range resp.Messages {
		out = append(out, q.SummariseMessage(&resp.Messages[i]))
	}
	return q.Result(fmt.Sprintf("Got %d message(s) from the queue", len(out)), map[string]interface{}{
		"messages": out, "count": fmt.Sprintf("%d", len(out)),
	}), nil
}
