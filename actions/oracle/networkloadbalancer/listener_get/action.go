// Package oracle_networkloadbalancer_listener_get reads one listener of a network
// load balancer by name.
package oracle_networkloadbalancer_listener_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Get Listener"
	Description  = "Fetch one listener of an Oracle Cloud network load balancer by name — its protocol, port and default backend set."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name", Placeholder: "The listener name, e.g. http-listener", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "listener", Type: core.ConnectionTypeObject, Label: "Listener"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Listener Name"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, nlbID, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := nlbn.RequiredString("listener_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetListener(nlbn.Context(), nlb.GetListenerRequest{NetworkLoadBalancerId: &nlbID, ListenerName: &name})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	summary := nlbn.SummariseListener(&resp.Listener)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Listener %q (%s)", summary["name"], summary["protocol"]),
		"listener":    summary,
		"name":        summary["name"],
		"success":     true,
	}, nil
}
