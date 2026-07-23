// Package oracle_queue_message_delete_batch deletes several messages from a queue in one call.
// The operator supplies the queue OCID and a comma-separated list of receipts (each returned by a
// prior Get Messages); this action resolves the queue's own messages endpoint automatically and
// posts them all at once. Per-receipt results distinguish which deletes succeeded and which failed.
package oracle_queue_message_delete_batch

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
	Name         = "OCI Queue: Delete Messages"
	Description  = "Delete several messages from a queue at once. Supply the queue OCID and a comma-separated list of receipts; the queue's endpoint is resolved automatically."
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
	{Name: "receipts", Type: core.ConnectionTypeText, Label: "Receipts", Placeholder: "Comma-separated receipts, one per message to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "server_failures", Type: core.ConnectionTypeInteger, Label: "Server Failures"},
	{Name: "client_failures", Type: core.ConnectionTypeInteger, Label: "Client Failures"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Per-message results"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queueID, err := q.RequiredString("queue_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	raw, err := q.RequiredString("receipts", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}
	// A receipt is an opaque token with no commas of its own, so a comma-split is safe. Trim each
	// and drop blanks so a trailing comma or stray whitespace never becomes an empty entry (which
	// the service would reject).
	var receipts []string
	for _, part := range strings.Split(raw, ",") {
		if r := strings.TrimSpace(part); r != "" {
			receipts = append(receipts, r)
		}
	}
	if len(receipts) == 0 {
		return q.ErrorResult("Receipts is required — supply at least one receipt"), nil
	}

	auth, client, errResult := q.DataPlaneClientForQueue(inputs, queueID)
	if errResult != nil {
		return errResult, nil
	}

	entries := make([]queue.DeleteMessagesDetailsEntry, 0, len(receipts))
	for i := range receipts {
		entries = append(entries, queue.DeleteMessagesDetailsEntry{Receipt: &receipts[i]})
	}

	resp, err := client.DeleteMessages(q.Context(), queue.DeleteMessagesRequest{
		QueueId:               &queueID,
		DeleteMessagesDetails: queue.DeleteMessagesDetails{Entries: entries},
	})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}

	// The result entries are guaranteed to be in the same order as the request. A successful delete
	// comes back as an empty entry; a failure carries errorCode/errorMessage.
	results := make([]map[string]interface{}, 0, len(resp.Entries))
	for i := range resp.Entries {
		e := resp.Entries[i]
		row := map[string]interface{}{"deleted": e.ErrorCode == nil && e.ErrorMessage == nil}
		if i < len(receipts) {
			row["receipt"] = receipts[i]
		}
		row["error_code"] = q.IntOrNil(e.ErrorCode)
		row["error_message"] = q.Str(e.ErrorMessage)
		results = append(results, row)
	}

	return q.Result(fmt.Sprintf("Deleted %d of %d message(s) — %v server failure(s), %v client failure(s)",
		len(receipts)-derefInt(resp.ServerFailures)-derefInt(resp.ClientFailures), len(receipts),
		q.IntOrNil(resp.ServerFailures), q.IntOrNil(resp.ClientFailures)),
		map[string]interface{}{
			"server_failures": q.IntOrNil(resp.ServerFailures),
			"client_failures": q.IntOrNil(resp.ClientFailures),
			"results":         results,
			"count":           len(results),
		}), nil
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
