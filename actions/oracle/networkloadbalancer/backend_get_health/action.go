// Package oracle_networkloadbalancer_backend_get_health reads the health status of
// one backend server within a backend set of a network load balancer.
package oracle_networkloadbalancer_backend_get_health

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Get Backend Health"
	Description  = "Read the health status of one backend server in a network load balancer backend set — its overall status and recent health-check results."
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
	{Name: "backend_name", Type: core.ConnectionTypeString, Label: "Backend Name", Placeholder: "The backend name, or <ipAddress>:<port> / <targetId>:<port>", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health", Type: core.ConnectionTypeObject, Label: "Backend Health"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Health Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, nlbID, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	backendSetName, err := nlbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	backendName, err := nlbn.RequiredString("backend_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetBackendHealth(nlbn.Context(), nlb.GetBackendHealthRequest{
		NetworkLoadBalancerId: &nlbID,
		BackendSetName:        &backendSetName,
		BackendName:           &backendName,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}

	results := make([]map[string]interface{}, 0, len(resp.HealthCheckResults))
	for i := range resp.HealthCheckResults {
		r := resp.HealthCheckResults[i]
		results = append(results, map[string]interface{}{
			"timestamp":           nlbn.FormatTime(r.Timestamp),
			"health_check_status": string(r.HealthCheckStatus),
		})
	}
	health := map[string]interface{}{
		"status":               string(resp.Status),
		"health_check_results": results,
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Backend %q in set %q health: %s (%d check result(s))", backendName, backendSetName, health["status"], len(results)),
		"health":      health,
		"status":      health["status"],
		"success":     true,
	}, nil
}
