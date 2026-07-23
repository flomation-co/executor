// Package oracle_queue_message_put puts a message onto a queue. The operator supplies the queue
// OCID and the message content; this action resolves the queue's own messages endpoint
// automatically and posts to it, so no raw endpoint URL is ever needed.
package oracle_queue_message_put

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Put Message"
	Description  = "Put a message onto a queue. Supply the queue OCID and the message content; the queue's endpoint is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "queue_ocid", Type: core.ConnectionTypeString, Label: "Queue OCID", Placeholder: "ocid1.queue.oc1..aaaa…", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Message Content", Placeholder: "The message body (any UTF-8 text or JSON)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
	{Name: "expire_after", Type: core.ConnectionTypeString, Label: "Expires After"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-message results"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	// Read the content WITHOUT trimming — a message body is opaque and whitespace/newlines are
	// significant. Only an entirely empty body is rejected.
	content := q.OptionalString("content", inputs)
	if content == "" {
		return q.ErrorResult("Message Content is required"), nil
	}
	auth, client, errResult := q.DataPlaneClientForQueue(inputs, queueID)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.PutMessages(q.Context(), queue.PutMessagesRequest{
		QueueId:            &queueID,
		PutMessagesDetails: queue.PutMessagesDetails{Messages: []queue.PutMessagesDetailsEntry{{Content: &content}}},
	})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	results := make([]map[string]interface{}, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		results = append(results, map[string]interface{}{"id": q.Int64OrNil(m.Id), "expire_after": q.FormatTime(m.ExpireAfter)})
	}
	first := map[string]interface{}{}
	if len(results) > 0 {
		first = results[0]
	}
	return q.Result(fmt.Sprintf("Put message %v onto the queue", first["id"]), map[string]interface{}{
		"id": first["id"], "expire_after": first["expire_after"], "results": results,
	}), nil
}
