// Package oracle_queue_queue_purge purges messages from a queue — clearing the normal queue, the
// dead-letter queue, or both, optionally scoped to specific channels. Asynchronous: it returns a
// work-request id you can poll while the purge completes.
package oracle_queue_queue_purge

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Purge Queue"
	Description  = "Delete messages from a queue — the normal queue, the dead-letter queue, or both, optionally scoped to specific channels. Returns a work-request id to poll."
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
	{Name: "purge_type", Type: core.ConnectionTypeString, Label: "Purge Type", Placeholder: "Which messages to delete", Required: true, Options: []core.ConnectionOption{
		{Name: "Normal queue only", Value: "NORMAL"},
		{Name: "Dead-letter queue only", Value: "DLQ"},
		{Name: "Both", Value: "BOTH"},
	}},
	{Name: "channel_ids", Type: core.ConnectionTypeString, Label: "Channel IDs", Placeholder: "Comma-separated channel IDs to limit the purge to (optional — omit to purge the whole queue)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Queue OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := q.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	rawType, err := q.RequiredString("purge_type", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	purgeType, ok := queue.GetMappingPurgeQueueDetailsPurgeTypeEnum(rawType)
	if !ok {
		return q.ErrorResult(fmt.Sprintf("purge type %q is not valid — choose one of %s", rawType, strings.Join(queue.GetPurgeQueueDetailsPurgeTypeEnumStringValues(), ", "))), nil
	}

	details := queue.PurgeQueueDetails{PurgeType: purgeType}
	if raw := strings.TrimSpace(q.OptionalString("channel_ids", inputs)); raw != "" {
		var channels []string
		for _, part := range strings.Split(raw, ",") {
			if id := strings.TrimSpace(part); id != "" {
				channels = append(channels, id)
			}
		}
		details.ChannelIds = channels
	}

	resp, err := client.PurgeQueue(q.Context(), queue.PurgeQueueRequest{QueueId: &queueID, PurgeQueueDetails: details})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	return q.Result(fmt.Sprintf("Purging queue (%s) — poll the work request until it completes", string(purgeType)), map[string]interface{}{
		"id":              queueID,
		"work_request_id": q.Str(resp.OpcWorkRequestId),
	}), nil
}
