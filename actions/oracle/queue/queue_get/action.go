// Package oracle_queue_queue_get fetches one queue by OCID from the regional control plane and
// returns its full configuration — lifecycle state, retention/visibility/timeout settings and,
// importantly, the queue's own messages endpoint (the host the data-plane message actions target).
package oracle_queue_queue_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Get Queue"
	Description  = "Fetch a queue by OCID. Returns its lifecycle state, settings and messages endpoint."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "queue", Type: core.ConnectionTypeObject, Label: "Queue"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Queue OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "messages_endpoint", Type: core.ConnectionTypeString, Label: "Messages Endpoint"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := q.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.GetQueue(q.Context(), queue.GetQueueRequest{QueueId: &queueID})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	summary := q.SummariseQueue(&resp.Queue)
	return q.Result(fmt.Sprintf("Queue %q is %s", q.Str(resp.DisplayName), string(resp.LifecycleState)), map[string]interface{}{
		"queue":             summary,
		"id":                q.Str(resp.Id),
		"lifecycle_state":   string(resp.LifecycleState),
		"messages_endpoint": q.Str(resp.MessagesEndpoint),
	}), nil
}
