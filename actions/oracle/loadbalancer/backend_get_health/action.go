// Package oracle_loadbalancer_backend_get_health reads the health status of one
// backend server of a load balancer's backend set.
package oracle_loadbalancer_backend_get_health

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Get Backend Health"
	Description  = "Read one backend server's health in an Oracle Cloud load balancer — overall status plus the most recent per-load-balancer health check results."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set name, e.g. web-servers", Required: true},
	{Name: "backend_name", Type: core.ConnectionTypeString, Label: "Backend Name", Placeholder: "The backend server IP:port, e.g. 10.0.0.3:8080", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health", Type: core.ConnectionTypeObject, Label: "Backend Health"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Health Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	setName, err := lbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	backendName, err := lbn.RequiredString("backend_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// SYNC read: GetBackendHealth returns the BackendHealth body inline (no work request).
	resp, err := client.GetBackendHealth(lbn.Context(), lb.GetBackendHealthRequest{
		LoadBalancerId: &lbID,
		BackendSetName: &setName,
		BackendName:    &backendName,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}

	status := string(resp.BackendHealth.Status)
	results := make([]map[string]interface{}, 0, len(resp.BackendHealth.HealthCheckResults))
	for i := range resp.BackendHealth.HealthCheckResults {
		r := resp.BackendHealth.HealthCheckResults[i]
		results = append(results, map[string]interface{}{
			"subnet_id":           lbn.Str(r.SubnetId),
			"source_ip_address":   lbn.Str(r.SourceIpAddress),
			"health_check_status": string(r.HealthCheckStatus),
			"timestamp":           lbn.FormatTime(r.Timestamp),
		})
	}
	health := map[string]interface{}{
		"status":               status,
		"health_check_results": results,
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Backend %q health: %s (%d check result(s))", backendName, status, len(results)),
		"health":      health,
		"status":      status,
		"success":     true,
	}, nil
}
