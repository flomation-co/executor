// Package oracle_loadbalancer_ssl_cipher_suite_update replaces the cipher list of a
// custom SSL cipher suite on an Oracle Cloud load balancer. Cipher suites are keyed by
// name within the load balancer; Oracle's reserved oci-* suites cannot be updated.
// Asynchronous — returns a work-request id.
package oracle_loadbalancer_ssl_cipher_suite_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Update SSL Cipher Suite"
	Description  = "Replace the cipher list of a custom SSL cipher suite on an Oracle Cloud load balancer (Oracle's reserved oci-* suites cannot be updated). Asynchronous."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+lock"
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
	{Name: "cipher_suite_name", Type: core.ConnectionTypeString, Label: "Cipher Suite Name", Placeholder: "The name of the custom cipher suite to update, e.g. example_cipher_suite", Required: true},
	{Name: "ciphers", Type: core.ConnectionTypeString, Label: "Ciphers", Placeholder: "Comma-separated cipher list, e.g. ECDHE-RSA-AES256-GCM-SHA384,ECDHE-ECDSA-AES256-GCM-SHA384 (replaces the whole list)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cipher_suite_name", Type: core.ConnectionTypeString, Label: "Cipher Suite Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := lbn.RequiredString("cipher_suite_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	ciphers := lbn.InputStrings("ciphers", inputs)
	if len(ciphers) == 0 {
		return lbn.ErrorResult("ciphers is required — supply a comma-separated cipher list"), nil
	}
	// Replace-semantics: UpdateSslCipherSuiteDetails overwrites the whole cipher list.
	resp, err := client.UpdateSSLCipherSuite(lbn.Context(), lb.UpdateSSLCipherSuiteRequest{
		LoadBalancerId: &lbID,
		Name:           &name,
		UpdateSslCipherSuiteDetails: lb.UpdateSslCipherSuiteDetails{
			Ciphers: ciphers,
		},
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Updating SSL cipher suite %q on load balancer %s — poll work request %s", name, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"cipher_suite_name": name,
		"work_request_id":   lbn.Str(resp.OpcWorkRequestId),
		"success":           true,
	}, nil
}
