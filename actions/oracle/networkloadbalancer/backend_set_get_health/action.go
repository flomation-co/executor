// Package oracle_networkloadbalancer_backend_set_get_health reads the health
// status of one backend set of a network load balancer by name.
package oracle_networkloadbalancer_backend_set_get_health

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Get Backend Set Health"
	Description  = "Read the health of an Oracle Cloud network load balancer backend set — overall status and the warning, critical and unknown backend server lists."
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
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set name, e.g. app-servers", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health", Type: core.ConnectionTypeObject, Label: "Backend Set Health"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Health Status"},
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
	resp, err := client.GetBackendSetHealth(nlbn.Context(), nlb.GetBackendSetHealthRequest{NetworkLoadBalancerId: &nlbID, BackendSetName: &name})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}

	h := resp.BackendSetHealth
	warning := h.WarningStateBackendNames
	critical := h.CriticalStateBackendNames
	unknown := h.UnknownStateBackendNames
	if warning == nil {
		warning = []string{}
	}
	if critical == nil {
		critical = []string{}
	}
	if unknown == nil {
		unknown = []string{}
	}
	total := 0
	if h.TotalBackendCount != nil {
		total = *h.TotalBackendCount
	}
	health := map[string]interface{}{
		"status":                       string(h.Status),
		"total_backend_count":          total,
		"warning_count":                len(warning),
		"critical_count":               len(critical),
		"unknown_count":                len(unknown),
		"warning_state_backend_names":  warning,
		"critical_state_backend_names": critical,
		"unknown_state_backend_names":  unknown,
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Backend set %q health: %s (%d backends)", name, string(h.Status), total),
		"health":      health,
		"status":      string(h.Status),
		"success":     true,
	}, nil
}
