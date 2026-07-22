// Package oracle_networkloadbalancer_network_load_balancer_get reads one network load
// balancer by OCID.
package oracle_networkloadbalancer_network_load_balancer_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Get Network Load Balancer"
	Description  = "Fetch a single Oracle Cloud network load balancer by OCID — its subnet, IP addresses, and the names of its listeners and backend sets."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+ethernet"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_load_balancer", Type: core.ConnectionTypeObject, Label: "Network Load Balancer"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetNetworkLoadBalancer(nlbn.Context(), nlb.GetNetworkLoadBalancerRequest{NetworkLoadBalancerId: &id})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	summary := nlbn.SummariseNetworkLoadBalancer(&resp.NetworkLoadBalancer)
	return map[string]interface{}{
		"tool_result":           fmt.Sprintf("Network load balancer %q is %s", summary["display_name"], summary["lifecycle_state"]),
		"network_load_balancer": summary,
		"id":                    summary["id"],
		"lifecycle_state":       summary["lifecycle_state"],
		"success":               true,
	}, nil
}
