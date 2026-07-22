// Package oracle_queue_message_delete deletes a single message from a queue. The receipt comes from
// a prior Get Messages call (a message is only deletable while it holds a valid receipt); this
// action resolves the queue's own messages endpoint automatically, so no raw endpoint URL is needed.
package oracle_queue_message_delete

import (
	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Delete Message"
	Description  = "Delete a single message from a queue using the receipt returned by a prior Get Messages call."
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
	{Name: "receipt", Type: core.ConnectionTypeText, Label: "Message Receipt", Placeholder: "The receipt returned by a prior Get Messages call", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	auth, client, errResult := q.DataPlaneClientForQueue(inputs, queueID)
	if errResult != nil {
		return errResult, nil
	}

	_, err = client.DeleteMessage(q.Context(), queue.DeleteMessageRequest{
		QueueId:        &queueID,
		MessageReceipt: &receipt,
	})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	return q.Result("Deleted the message from the queue", nil), nil
}
