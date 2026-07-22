// Package oracle_queue_message_update_batch updates the visibility of several in-flight messages on
// a queue in one call. The operator supplies the queue OCID and a JSON array of entries, each with
// the message's receipt and a new visibility timeout; the queue's own messages endpoint is resolved
// automatically. Per-message outcomes are reported individually so a partial success is visible.
package oracle_queue_message_update_batch

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Update Messages (Batch)"
	Description  = "Update the visibility of several in-flight messages at once. Supply the queue OCID and a JSON array of entries, each with a message receipt and a new visibility timeout in seconds."
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
	{Name: "entries_json", Type: core.ConnectionTypeText, Label: "Entries (JSON)", Placeholder: "[{\"receipt\":\"…\",\"visibilityInSeconds\":30}]", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "server_failures", Type: core.ConnectionTypeInteger, Label: "Server Failures"},
	{Name: "client_failures", Type: core.ConnectionTypeInteger, Label: "Client Failures"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-message results"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// entryInput mirrors one element of the entries JSON array. Pointers let us tell "absent" from a
// zero value so both mandatory fields can be enforced before the call.
type entryInput struct {
	Receipt             *string `json:"receipt"`
	VisibilityInSeconds *int    `json:"visibilityInSeconds"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	raw := strings.TrimSpace(q.OptionalString("entries_json", inputs))
	if raw == "" {
		return q.ErrorResult("Entries (JSON) is required"), nil
	}
	var parsed []entryInput
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return q.ErrorResult(fmt.Sprintf("Entries (JSON) must be a JSON array of {\"receipt\":\"…\",\"visibilityInSeconds\":N} objects: %s", err.Error())), nil
	}
	if len(parsed) == 0 {
		return q.ErrorResult("Entries (JSON) must contain at least one entry"), nil
	}
	entries := make([]queue.UpdateMessagesDetailsEntry, 0, len(parsed))
	for i := range parsed {
		e := parsed[i]
		if e.Receipt == nil || strings.TrimSpace(*e.Receipt) == "" {
			return q.ErrorResult(fmt.Sprintf("entry %d is missing a receipt", i+1)), nil
		}
		if e.VisibilityInSeconds == nil {
			return q.ErrorResult(fmt.Sprintf("entry %d is missing visibilityInSeconds", i+1)), nil
		}
		entries = append(entries, queue.UpdateMessagesDetailsEntry{Receipt: e.Receipt, VisibilityInSeconds: e.VisibilityInSeconds})
	}

	auth, client, errResult := q.DataPlaneClientForQueue(inputs, queueID)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.UpdateMessages(q.Context(), queue.UpdateMessagesRequest{
		QueueId:               &queueID,
		UpdateMessagesDetails: queue.UpdateMessagesDetails{Entries: entries},
	})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}

	results := make([]map[string]interface{}, 0, len(resp.Entries))
	for i := range resp.Entries {
		r := resp.Entries[i]
		results = append(results, map[string]interface{}{
			"id":            q.Int64OrNil(r.Id),
			"visible_after": q.FormatTime(r.VisibleAfter),
			"error_code":    q.IntOrNil(r.ErrorCode),
			"error_message": q.Str(r.ErrorMessage),
		})
	}
	serverFailures, clientFailures := 0, 0
	if resp.ServerFailures != nil {
		serverFailures = *resp.ServerFailures
	}
	if resp.ClientFailures != nil {
		clientFailures = *resp.ClientFailures
	}
	return q.Result(fmt.Sprintf("Updated %d message(s) — %d server failure(s), %d client failure(s)", len(entries), serverFailures, clientFailures), map[string]interface{}{
		"server_failures": q.IntOrNil(resp.ServerFailures),
		"client_failures": q.IntOrNil(resp.ClientFailures),
		"results":         results,
		"count":           fmt.Sprintf("%d", len(results)),
	}), nil
}
