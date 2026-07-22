// Package oracle_streaming_work_request_list lists the asynchronous work requests in a
// compartment — the activity logs Streaming raises for long-running control-plane operations
// (create/update/delete stream, pool or connect harness). Walks pagination up to a safe cap.
package oracle_streaming_work_request_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: List Work Requests"
	Description  = "List the asynchronous work requests in a compartment — each tracks a long-running Streaming operation with its operation type, status and percent complete. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
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
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page, 1–50 (default 10)"},
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
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	req := streaming.ListWorkRequestsRequest{CompartmentId: &compartment}
	if n, ok, err := str.OptionalInt("limit", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= str.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkRequests(str.Context(), req)
		if err != nil {
			return str.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			w := &resp.Items[i]
			var percent interface{}
			if w.PercentComplete != nil {
				percent = *w.PercentComplete
			}
			out = append(out, map[string]interface{}{
				"id":               str.Str(w.Id),
				"operation_type":   string(w.OperationType),
				"status":           string(w.Status),
				"compartment_id":   str.Str(w.CompartmentId),
				"percent_complete": percent,
				"time_accepted":    str.FormatTime(w.TimeAccepted),
				"time_started":     str.FormatTime(w.TimeStarted),
				"time_finished":    str.FormatTime(w.TimeFinished),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return str.Result(fmt.Sprintf("Found %d work request(s)", len(out)), map[string]interface{}{
		"work_requests": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
