// Package oracle_networkloadbalancer_backend_set_get reads one backend set of a
// network load balancer by name.
package oracle_networkloadbalancer_backend_set_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Get Backend Set"
	Description  = "Fetch one backend set of an Oracle Cloud network load balancer by name — its policy, health checker and backend servers."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set name, e.g. app-servers", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backend_set", Type: core.ConnectionTypeObject, Label: "Backend Set"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Backend Set Name"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, nlbID, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := nlbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetBackendSet(nlbn.Context(), nlb.GetBackendSetRequest{NetworkLoadBalancerId: &nlbID, BackendSetName: &name})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	summary := nlbn.SummariseBackendSet(&resp.BackendSet)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Backend set %q (%s)", summary["name"], summary["policy"]),
		"backend_set": summary,
		"name":        summary["name"],
		"success":     true,
	}, nil
}
