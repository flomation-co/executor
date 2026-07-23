// Package oracle_queue_queue_create creates a queue — the lightweight, managed message queue that
// producers put messages onto and consumers get them from. Asynchronous: the queue comes back
// CREATING with a work-request id; poll the Get Queue action until it is ACTIVE before use.
package oracle_queue_queue_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Create Queue"
	Description  = "Create a queue. Set an optional retention, visibility timeout and dead-letter delivery count. Returns the queue in a CREATING state plus a work-request id — poll Get Queue until ACTIVE."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the queue", Required: true},
	{Name: "retention_seconds", Type: core.ConnectionTypeString, Label: "Retention (seconds)", Placeholder: "How long to keep messages (optional)"},
	{Name: "visibility_seconds", Type: core.ConnectionTypeString, Label: "Visibility Timeout (seconds)", Placeholder: "How long a got message stays hidden (optional)"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeString, Label: "Long-poll Timeout (seconds)", Placeholder: "Default GET long-poll wait (optional)"},
	{Name: "dlq_delivery_count", Type: core.ConnectionTypeString, Label: "Dead-letter Delivery Count", Placeholder: "Deliveries before a message goes to the DLQ (optional)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "queue", Type: core.ConnectionTypeObject, Label: "Queue"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Queue OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := q.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	name, err := q.RequiredString("display_name", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}

	details := queue.CreateQueueDetails{DisplayName: &name, CompartmentId: &compartment}
	for field, set := range map[string]func(int){
		"retention_seconds":  func(n int) { details.RetentionInSeconds = &n },
		"visibility_seconds": func(n int) { details.VisibilityInSeconds = &n },
		"timeout_seconds":    func(n int) { details.TimeoutInSeconds = &n },
		"dlq_delivery_count": func(n int) { details.DeadLetterQueueDeliveryCount = &n },
	} {
		if n, ok, err := q.OptionalInt(field, inputs); err != nil {
			return q.ErrorResult(err.Error()), nil
		} else if ok {
			set(n)
		}
	}
	if tags, err := q.FreeformTags("freeform_tags", inputs); err != nil {
		return q.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.CreateQueue(q.Context(), queue.CreateQueueRequest{CreateQueueDetails: details})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	return q.Result(fmt.Sprintf("Creating queue %q — poll Get Queue until ACTIVE", name), map[string]interface{}{
		"queue":           map[string]interface{}{"display_name": name, "compartment_id": compartment},
		"lifecycle_state": "CREATING",
		"work_request_id": q.Str(resp.OpcWorkRequestId),
	}), nil
}
