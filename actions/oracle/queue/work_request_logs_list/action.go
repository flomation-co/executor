// Package oracle_queue_work_request_logs_list lists the log messages recorded against a Queue work
// request — the asynchronous request id returned by create/update/delete/purge operations. Use it
// to follow the progress of an asynchronous operation. Walks pagination up to a safe cap.
package oracle_queue_work_request_logs_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: List Work Request Logs"
	Description  = "List the log messages recorded against a Queue work request, so you can follow the progress of an asynchronous create, update, delete or purge. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "work_request_ocid", Type: core.ConnectionTypeString, Label: "Work Request OCID", Placeholder: "ocid1.queueworkrequest.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "logs", Type: core.ConnectionTypeObject, Label: "Logs"},
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
	workRequestID, err := q.RequiredString("work_request_ocid", inputs)
	if err != nil {
		return q.ErrorResult(err.Error()), nil
	}

	req := queue.ListWorkRequestLogsRequest{WorkRequestId: &workRequestID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= q.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkRequestLogs(q.Context(), req)
		if err != nil {
			return q.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			e := resp.Items[i]
			out = append(out, map[string]interface{}{
				"message":   q.Str(e.Message),
				"timestamp": q.FormatTime(e.Timestamp),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return q.Result(fmt.Sprintf("Found %d work-request log(s)", len(out)), map[string]interface{}{
		"logs": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
