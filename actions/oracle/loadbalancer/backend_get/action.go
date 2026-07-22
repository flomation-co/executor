// Package oracle_loadbalancer_backend_get reads one backend server of a load
// balancer's backend set by its "ip:port" name.
package oracle_loadbalancer_backend_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Get Backend"
	Description  = "Fetch one backend server of an Oracle Cloud load balancer's backend set by its ip:port name — its weight, ports and drain/backup/offline flags."
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
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set name, e.g. web-servers", Required: true},
	{Name: "backend_name", Type: core.ConnectionTypeString, Label: "Backend Name", Placeholder: "The backend server ip:port, e.g. 10.0.0.3:8080", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backend", Type: core.ConnectionTypeObject, Label: "Backend"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Backend Name"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	backendSetName, err := lbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	backendName, err := lbn.RequiredString("backend_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetBackend(lbn.Context(), lb.GetBackendRequest{LoadBalancerId: &lbID, BackendSetName: &backendSetName, BackendName: &backendName})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	summary := lbn.SummariseBackend(&resp.Backend)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Backend %q of backend set %q", summary["name"], backendSetName),
		"backend":     summary,
		"name":        summary["name"],
		"success":     true,
	}, nil
}
