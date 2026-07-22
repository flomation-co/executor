// Package oracle_queue_work_request_list lists the asynchronous work requests in a compartment —
// the queue control-plane operations (create/update/delete/move/purge) each spawn one, and this
// action surfaces their type, status and progress. Walks pagination up to a safe cap.
package oracle_queue_work_request_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: List Work Requests"
	Description  = "List the asynchronous work requests in a compartment, with each one's operation type, status and progress. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max work requests per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_requests", Type: core.ConnectionTypeObject, Label: "Work Requests"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	req := queue.ListWorkRequestsRequest{CompartmentId: &compartment}
	if n, ok, err := q.OptionalInt("limit", inputs); err != nil {
		return q.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= q.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkRequests(q.Context(), req)
		if err != nil {
			return q.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseWorkRequest(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return q.Result(fmt.Sprintf("Found %d work request(s)", len(out)), map[string]interface{}{
		"work_requests": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}

func summariseWorkRequest(w *queue.WorkRequestSummary) map[string]interface{} {
	var pct interface{}
	if w.PercentComplete != nil {
		pct = *w.PercentComplete
	}
	resources := make([]map[string]interface{}, 0, len(w.Resources))
	for i := range w.Resources {
		r := &w.Resources[i]
		resources = append(resources, map[string]interface{}{
			"entity_type": q.Str(r.EntityType),
			"action_type": string(r.ActionType),
			"identifier":  q.Str(r.Identifier),
			"entity_uri":  q.Str(r.EntityUri),
		})
	}
	return map[string]interface{}{
		"id":               q.Str(w.Id),
		"operation_type":   string(w.OperationType),
		"status":           string(w.Status),
		"compartment_id":   q.Str(w.CompartmentId),
		"percent_complete": pct,
		"time_accepted":    q.FormatTime(w.TimeAccepted),
		"time_started":     q.FormatTime(w.TimeStarted),
		"time_finished":    q.FormatTime(w.TimeFinished),
		"resources":        resources,
	}
}
