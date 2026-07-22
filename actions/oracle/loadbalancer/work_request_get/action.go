// Package oracle_loadbalancer_work_request_get reads the status of a load-balancer
// work request. Every asynchronous action returns a work-request id; poll it here
// until lifecycle_state is SUCCEEDED (or FAILED, with the message explaining why).
package oracle_loadbalancer_work_request_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Get Work Request"
	Description  = "Read the status of an Oracle Cloud load-balancer work request by OCID. Every asynchronous action returns a work-request id — poll it here until it is SUCCEEDED or FAILED."
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
	{Name: "work_request_ocid", Type: core.ConnectionTypeString, Label: "Work Request OCID", Placeholder: "ocid1.loadbalancerworkrequest.oc1..aaaa… (from any async action)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request", Type: core.ConnectionTypeObject, Label: "Work Request"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "load_balancer_id", Type: core.ConnectionTypeString, Label: "Load Balancer OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := lbn.ResourceClient(inputs, "work_request_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetWorkRequest(lbn.Context(), lb.GetWorkRequestRequest{WorkRequestId: &id})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	wr := lbn.SummariseWorkRequest(&resp.WorkRequest)
	msg := fmt.Sprintf("Work request %s is %s", wr["type"], wr["lifecycle_state"])
	if s, _ := wr["lifecycle_state"].(string); s == "FAILED" {
		msg = fmt.Sprintf("Work request %s FAILED: %s", wr["type"], wr["message"])
	}
	return map[string]interface{}{
		"tool_result":      msg,
		"work_request":     wr,
		"lifecycle_state":  wr["lifecycle_state"],
		"load_balancer_id": wr["load_balancer_id"],
		"success":          true,
	}, nil
}
