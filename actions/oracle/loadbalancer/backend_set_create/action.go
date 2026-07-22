// Package oracle_loadbalancer_backend_set_create adds a backend set (a pool of
// backend servers with a load-balancing policy and a health check) to a load
// balancer. Asynchronous — returns a work-request id.
package oracle_loadbalancer_backend_set_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Create Backend Set"
	Description  = "Add a backend set — a pool of backend servers with a load-balancing policy (e.g. ROUND_ROBIN) and a health check — to an Oracle Cloud load balancer. Asynchronous."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "A name unique within the load balancer, e.g. web-servers", Required: true},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Policy", Placeholder: "ROUND_ROBIN, LEAST_CONNECTIONS or IP_HASH", Required: true},
	{Name: "health_check_protocol", Type: core.ConnectionTypeString, Label: "Health Check Protocol", Placeholder: "HTTP or TCP", Required: true},
	{Name: "health_check_port", Type: core.ConnectionTypeString, Label: "Health Check Port", Placeholder: "e.g. 80 (0 = use each backend's port)"},
	{Name: "health_check_url_path", Type: core.ConnectionTypeString, Label: "Health Check URL Path", Placeholder: "For HTTP checks, e.g. /health"},
	{Name: "health_check_return_code", Type: core.ConnectionTypeString, Label: "Health Check Return Code", Placeholder: "Expected HTTP status, e.g. 200"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := lbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	policy, err := lbn.RequiredString("policy", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	if policy, err = lbn.ValidateEnum("policy", policy, lbn.BackendPolicies...); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	hcProtocol, err := lbn.RequiredString("health_check_protocol", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	if hcProtocol, err = lbn.ValidateEnum("health check protocol", hcProtocol, lbn.HealthCheckProtocols...); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	hc := &lb.HealthCheckerDetails{Protocol: &hcProtocol}
	if v, ok, err := lbn.OptionalInt("health_check_port", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		hc.Port = &v
	}
	if v := strings.TrimSpace(lbn.OptionalString("health_check_url_path", inputs)); v != "" {
		hc.UrlPath = &v
	}
	if v, ok, err := lbn.OptionalInt("health_check_return_code", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		hc.ReturnCode = &v
	}
	resp, err := client.CreateBackendSet(lbn.Context(), lb.CreateBackendSetRequest{
		LoadBalancerId: &lbID,
		CreateBackendSetDetails: lb.CreateBackendSetDetails{
			Name:          &name,
			Policy:        &policy,
			HealthChecker: hc,
		},
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Creating backend set %q on load balancer %s — poll work request %s", name, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"backend_set_name": name,
		"work_request_id":  lbn.Str(resp.OpcWorkRequestId),
		"success":          true,
	}, nil
}
