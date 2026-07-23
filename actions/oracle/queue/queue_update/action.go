// Package oracle_queue_queue_update applies a partial update to a queue: display name, visibility
// timeout, long-poll timeout, dead-letter delivery count, per-channel consumption limit and
// freeform tags. Only the fields you supply are changed. Asynchronous: the queue goes to UPDATING
// with a work-request id — poll Get Queue until it is ACTIVE again.
package oracle_queue_queue_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Update Queue"
	Description  = "Partially update a queue — change any of its display name, visibility timeout, long-poll timeout, dead-letter delivery count, channel consumption limit or freeform tags. Returns the queue in an UPDATING state plus a work-request id — poll Get Queue until ACTIVE."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A new name for the queue (leave blank to keep)"},
	{Name: "visibility_seconds", Type: core.ConnectionTypeString, Label: "Visibility Timeout (seconds)", Placeholder: "How long a got message stays hidden (leave blank to keep)"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeString, Label: "Long-poll Timeout (seconds)", Placeholder: "Default GET long-poll wait (leave blank to keep)"},
	{Name: "dlq_delivery_count", Type: core.ConnectionTypeString, Label: "Dead-letter Delivery Count", Placeholder: "Deliveries before a message goes to the DLQ (leave blank to keep)"},
	{Name: "channel_consumption_limit", Type: core.ConnectionTypeString, Label: "Channel Consumption Limit (%)", Placeholder: "Max percent of queue resources one channel may use, 0–100 (leave blank to keep)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} — replaces all freeform tags (leave blank to keep)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Queue OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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

	details := queue.UpdateQueueDetails{}
	if name := q.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	for field, set := range map[string]func(int){
		"visibility_seconds":        func(n int) { details.VisibilityInSeconds = &n },
		"timeout_seconds":           func(n int) { details.TimeoutInSeconds = &n },
		"dlq_delivery_count":        func(n int) { details.DeadLetterQueueDeliveryCount = &n },
		"channel_consumption_limit": func(n int) { details.ChannelConsumptionLimit = &n },
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

	resp, err := client.UpdateQueue(q.Context(), queue.UpdateQueueRequest{
		QueueId:            &queueID,
		UpdateQueueDetails: details,
	})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	return q.Result(fmt.Sprintf("Updating queue %s — poll Get Queue until ACTIVE", queueID), map[string]interface{}{
		"id":              queueID,
		"lifecycle_state": "UPDATING",
		"work_request_id": q.Str(resp.OpcWorkRequestId),
	}), nil
}
