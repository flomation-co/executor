// Package oracle_networkloadbalancer_network_load_balancer_get_health reads the overall
// health of one network load balancer by OCID.
package oracle_networkloadbalancer_network_load_balancer_get_health

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Get Network Load Balancer Health"
	Description  = "Read an Oracle Cloud network load balancer's overall health — its aggregate status plus the names of any backend sets currently in a warning, critical, or unknown state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
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
	{Name: "health", Type: core.ConnectionTypeObject, Label: "Health"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Health Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetNetworkLoadBalancerHealth(nlbn.Context(), nlb.GetNetworkLoadBalancerHealthRequest{NetworkLoadBalancerId: &id})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}

	h := resp.NetworkLoadBalancerHealth
	status := string(h.Status)
	warning := h.WarningStateBackendSetNames
	if warning == nil {
		warning = []string{}
	}
	critical := h.CriticalStateBackendSetNames
	if critical == nil {
		critical = []string{}
	}
	unknown := h.UnknownStateBackendSetNames
	if unknown == nil {
		unknown = []string{}
	}
	health := map[string]interface{}{
		"status":                           status,
		"warning_state_backend_set_names":  warning,
		"critical_state_backend_set_names": critical,
		"unknown_state_backend_set_names":  unknown,
	}
	if h.TotalBackendSetCount != nil {
		health["total_backend_set_count"] = *h.TotalBackendSetCount
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Network load balancer health is %s (%d warning, %d critical, %d unknown backend set(s))", status, len(warning), len(critical), len(unknown)),
		"health":      health,
		"status":      status,
		"success":     true,
	}, nil
}
