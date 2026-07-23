// Package oracle_loadbalancer_load_balancer_get_health reads a load balancer's
// overall health status — the aggregate OK/WARNING/CRITICAL/UNKNOWN roll-up plus the
// friendly names of any backend sets currently in a non-OK state.
package oracle_loadbalancer_load_balancer_get_health

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Get Load Balancer Health"
	Description  = "Read a load balancer's overall health status — the aggregate OK/WARNING/CRITICAL/UNKNOWN roll-up plus the names of any backend sets in a warning, critical, or unknown state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health", Type: core.ConnectionTypeObject, Label: "Health"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Health Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	// SYNC read: the response carries the LoadBalancerHealth body directly, no work request.
	resp, err := client.GetLoadBalancerHealth(lbn.Context(), lb.GetLoadBalancerHealthRequest{LoadBalancerId: &id})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	h := resp.LoadBalancerHealth
	status := string(h.Status)
	total := 0
	if h.TotalBackendSetCount != nil {
		total = *h.TotalBackendSetCount
	}
	health := map[string]interface{}{
		"status":                           status,
		"warning_state_backend_set_names":  h.WarningStateBackendSetNames,
		"critical_state_backend_set_names": h.CriticalStateBackendSetNames,
		"unknown_state_backend_set_names":  h.UnknownStateBackendSetNames,
		"total_backend_set_count":          total,
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Load balancer health is %s (%d backend sets)", status, total),
		"health":      health,
		"status":      status,
		"success":     true,
	}, nil
}
