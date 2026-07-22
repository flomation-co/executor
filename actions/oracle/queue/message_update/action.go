// Package oracle_queue_message_update extends (or shortens) the visibility timeout of an in-flight
// message. Supply the queue OCID, the receipt handed back by Get Messages, and the new visibility in
// seconds; the queue's own messages endpoint is resolved automatically, so no raw endpoint is needed.
package oracle_queue_message_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Update Message"
	Description  = "Extend or shorten a message's visibility timeout. Supply the queue OCID, the message receipt, and the new visibility in seconds relative to now."
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
	{Name: "receipt", Type: core.ConnectionTypeString, Label: "Message Receipt", Placeholder: "The receipt returned by Get Messages", Required: true},
	{Name: "visibility_seconds", Type: core.ConnectionTypeString, Label: "Visibility (seconds)", Placeholder: "New visibility timeout relative to now, in seconds", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
	{Name: "visible_after", Type: core.ConnectionTypeString, Label: "Visible After"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	receipt, err := q.RequiredString("receipt", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	visibility, err := q.RequiredInt("visibility_seconds", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := q.DataPlaneClientForQueue(inputs, queueID)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.UpdateMessage(q.Context(), queue.UpdateMessageRequest{
		QueueId:              &queueID,
		MessageReceipt:       &receipt,
		UpdateMessageDetails: queue.UpdateMessageDetails{VisibilityInSeconds: &visibility},
	})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	id := q.Int64OrNil(resp.Id)
	visibleAfter := q.FormatTime(resp.VisibleAfter)
	return q.Result(fmt.Sprintf("Updated message %v — now visible after %s", id, visibleAfter), map[string]interface{}{
		"id":            id,
		"visible_after": visibleAfter,
	}), nil
}
