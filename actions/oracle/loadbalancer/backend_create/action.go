// Package oracle_loadbalancer_backend_create adds a backend server (an IP:port) to a
// backend set. Asynchronous — returns a work-request id.
package oracle_loadbalancer_backend_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Create Backend"
	Description  = "Add a backend server (IP address and port) to a backend set of an Oracle Cloud load balancer. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set to add the server to, e.g. web-servers", Required: true},
	{Name: "ip_address", Type: core.ConnectionTypeString, Label: "IP Address", Placeholder: "The backend server's IP, e.g. 10.0.1.5", Required: true},
	{Name: "port", Type: core.ConnectionTypeString, Label: "Port", Placeholder: "The backend server's port, e.g. 8080", Required: true},
	{Name: "weight", Type: core.ConnectionTypeString, Label: "Weight", Placeholder: "Relative traffic weight, e.g. 1 (optional)"},
	{Name: "backup", Type: core.ConnectionTypeBoolean, Label: "Backup", Placeholder: "Only receive traffic when all non-backup backends are unhealthy (optional)"},
	{Name: "drain", Type: core.ConnectionTypeBoolean, Label: "Drain", Placeholder: "Stop sending NEW connections (drain existing) (optional)"},
	{Name: "offline", Type: core.ConnectionTypeBoolean, Label: "Offline", Placeholder: "Take the backend out of rotation entirely (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	bsName, err := lbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	ip, err := lbn.RequiredString("ip_address", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	port, err := lbn.RequiredInt("port", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	details := lb.CreateBackendDetails{IpAddress: &ip, Port: &port}
	if v, ok, err := lbn.OptionalInt("weight", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Weight = &v
	}
	for _, f := range []struct {
		name string
		set  func(*bool)
	}{
		{"backup", func(b *bool) { details.Backup = b }},
		{"drain", func(b *bool) { details.Drain = b }},
		{"offline", func(b *bool) { details.Offline = b }},
	} {
		if lbn.BoolWasSet(f.name, inputs) {
			b := lbn.OptionalBool(f.name, inputs, false)
			f.set(&b)
		}
	}
	resp, err := client.CreateBackend(lbn.Context(), lb.CreateBackendRequest{
		LoadBalancerId:       &lbID,
		BackendSetName:       &bsName,
		CreateBackendDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Adding backend %s:%d to backend set %q — poll work request %s", ip, port, bsName, lbn.Str(resp.OpcWorkRequestId)),
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
