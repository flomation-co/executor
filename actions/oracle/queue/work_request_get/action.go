// Package oracle_queue_work_request_get fetches one OCI Queue work request by OCID. Asynchronous
// control-plane operations (create/update/delete/move/purge a queue or consumer group) return a
// work-request id; this action reports its operation type, status and completion percentage so a
// flow can poll until it is done.
package oracle_queue_work_request_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	q "flomation.app/automate/executor/actions/oracle/queue"

	"github.com/oracle/oci-go-sdk/v65/queue"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Queue: Get Work Request"
	Description  = "Fetch an OCI Queue work request by OCID — reports its operation type, status and completion percentage so you can poll an async queue operation until it finishes."
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
	{Name: "work_request_ocid", Type: core.ConnectionTypeString, Label: "Work Request OCID", Placeholder: "ocid1.queueworkrequest.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request", Type: core.ConnectionTypeObject, Label: "Work Request"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "percent_complete", Type: core.ConnectionTypeString, Label: "Percent Complete"},
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

	resp, err := client.GetWorkRequest(q.Context(), queue.GetWorkRequestRequest{WorkRequestId: &workRequestID})
	if err != nil {
		return q.ErrorResult(auth.OCIError(err)), nil
	}
	wr := resp.WorkRequest

	var percent interface{}
	if wr.PercentComplete != nil {
		percent = float64(*wr.PercentComplete)
	}
	workRequest := map[string]interface{}{
		"id":               q.Str(wr.Id),
		"operation_type":   string(wr.OperationType),
		"status":           string(wr.Status),
		"compartment_id":   q.Str(wr.CompartmentId),
		"percent_complete": percent,
		"time_accepted":    q.FormatTime(wr.TimeAccepted),
		"time_started":     q.FormatTime(wr.TimeStarted),
		"time_finished":    q.FormatTime(wr.TimeFinished),
	}

	return q.Result(fmt.Sprintf("Work request %s is %s (%v%% complete)", string(wr.OperationType), string(wr.Status), percent), map[string]interface{}{
		"work_request":     workRequest,
		"status":           string(wr.Status),
		"percent_complete": percent,
	}), nil
}
