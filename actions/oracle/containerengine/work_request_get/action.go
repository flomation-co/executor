// Package oracle_containerengine_work_request_get reads an OKE work request by OCID — poll
// this after any create/update/delete to watch the operation through to completion.
package oracle_containerengine_work_request_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Get Work Request"
	Description  = "Fetch an Oracle Cloud OKE work request by OCID — its operation type, status (ACCEPTED/IN_PROGRESS/SUCCEEDED/FAILED), affected resources and timings. Poll this after any create/update/delete to track the async operation."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the work-request picker)"},
	{Name: "work_request_ocid", Type: core.ConnectionTypeString, Label: "Work Request OCID", Placeholder: "ocid1.clustersworkrequest.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request", Type: core.ConnectionTypeObject, Label: "Work Request"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := oke.RequiredString("work_request_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetWorkRequest(oke.Context(), okesdk.GetWorkRequestRequest{WorkRequestId: &id})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	wr := oke.SummariseWorkRequest(&resp.WorkRequest)
	return oke.Result(fmt.Sprintf("Work request %s is %s", wr["operation_type"], wr["status"]), map[string]interface{}{
		"work_request": wr, "id": wr["id"], "status": wr["status"],
	}), nil
}
