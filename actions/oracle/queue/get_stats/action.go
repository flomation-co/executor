// Package oracle_queue_get_stats reads the message statistics for a queue — the approximate count
// of visible and in-flight messages and the total size in bytes — for the queue as a whole, its
// dead-letter queue, and (optionally) a single channel. Data plane: the queue's own messages
// endpoint is resolved automatically from the queue OCID.
package oracle_queue_get_stats

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Get Stats"
	Description  = "Read the message statistics for a queue — visible and in-flight message counts and size in bytes — for the queue, its dead-letter queue, and optionally one channel."
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
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Placeholder: "Limit the stats to a single channel (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stats", Type: core.ConnectionTypeObject, Label: "Stats"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func summariseStats(s *queue.Stats) map[string]interface{} {
	if s == nil {
		return nil
	}
	return map[string]interface{}{
		"visible_messages":   q.Int64OrNil(s.VisibleMessages),
		"in_flight_messages": q.Int64OrNil(s.InFlightMessages),
		"size_in_bytes":      q.Int64OrNil(s.SizeInBytes),
	}
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

	req := queue.GetStatsRequest{QueueId: &queueID}
	if channel := q.OptionalString("channel_id", inputs); channel != "" {
		req.ChannelId = &channel
	}

	resp, err := client.GetStats(q.Context(), req)
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}

	stats := map[string]interface{}{
		"queue":             summariseStats(resp.Queue),
		"dlq":               summariseStats(resp.Dlq),
		"channel_id":        q.Str(resp.ChannelId),
		"consumer_group_id": q.Str(resp.ConsumerGroupId),
	}
	visible := interface{}(nil)
	inFlight := interface{}(nil)
	if resp.Queue != nil {
		visible = q.Int64OrNil(resp.Queue.VisibleMessages)
		inFlight = q.Int64OrNil(resp.Queue.InFlightMessages)
	}
	return q.Result(fmt.Sprintf("Queue has %v visible and %v in-flight message(s)", visible, inFlight), map[string]interface{}{
		"stats":      stats,
		"channel_id": q.Str(resp.ChannelId),
	}), nil
}
