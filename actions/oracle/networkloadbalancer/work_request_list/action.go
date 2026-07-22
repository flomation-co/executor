// Package oracle_networkloadbalancer_work_request_list lists the network-load-balancer
// work requests in a compartment. Every asynchronous action returns a work-request id;
// this walks the compartment's work-request history (up to a safe pagination cap).
package oracle_networkloadbalancer_work_request_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: List Work Requests"
	Description  = "List the Oracle Cloud network-load-balancer work requests in a compartment. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+clock"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_requests", Type: core.ConnectionTypeObject, Label: "Work Requests"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// summariseWorkRequestSummary mirrors nlbn.SummariseWorkRequest for the list endpoint,
// which returns the distinct (but field-compatible) WorkRequestSummary type.
func summariseWorkRequestSummary(w *nlb.WorkRequestSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":               nlbn.Str(w.Id),
		"operation_type":   string(w.OperationType),
		"status":           string(w.Status),
		"compartment_id":   nlbn.Str(w.CompartmentId),
		"percent_complete": w.PercentComplete,
		"time_accepted":    nlbn.FormatTime(w.TimeAccepted),
		"time_finished":    nlbn.FormatTime(w.TimeFinished),
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := nlbn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	req := nlb.ListWorkRequestsRequest{CompartmentId: &compartment}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= nlbn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkRequests(nlbn.Context(), req)
		if err != nil {
			return nlbn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseWorkRequestSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d work request(s)", len(out)),
		"work_requests": out,
		"count":         fmt.Sprintf("%d", len(out)),
		"truncated":     truncated,
		"success":       true,
	}, nil
}
