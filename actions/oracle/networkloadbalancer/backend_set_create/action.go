// Package oracle_networkloadbalancer_backend_set_create adds a backend set (a server
// pool with a tuple-hash policy and a health check) to a network load balancer.
// Asynchronous — returns a work-request id.
package oracle_networkloadbalancer_backend_set_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Create Backend Set"
	Description  = "Add a backend set — a server pool with a tuple-hash policy (TWO_TUPLE/THREE_TUPLE/FIVE_TUPLE) and a health check — to an Oracle Cloud network load balancer. Asynchronous."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "A name unique within the NLB, e.g. app-servers", Required: true},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Policy", Placeholder: "TWO_TUPLE, THREE_TUPLE or FIVE_TUPLE", Required: true},
	{Name: "health_check_protocol", Type: core.ConnectionTypeString, Label: "Health Check Protocol", Placeholder: "TCP, HTTP, HTTPS, UDP or DNS", Required: true},
	{Name: "health_check_port", Type: core.ConnectionTypeString, Label: "Health Check Port", Placeholder: "e.g. 80 (0 = use each backend's port)"},
	{Name: "health_check_url_path", Type: core.ConnectionTypeString, Label: "Health Check URL Path", Placeholder: "For HTTP/HTTPS checks, e.g. /health"},
	{Name: "is_preserve_source", Type: core.ConnectionTypeBoolean, Label: "Preserve Source IP", Placeholder: "Pass the client's source IP to the backend (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	policyRaw, err := nlbn.RequiredString("policy", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	policy, err := nlbn.ValidateEnum("policy", policyRaw, nlbn.NlbPolicies...)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	hcpRaw, err := nlbn.RequiredString("health_check_protocol", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	hcp, err := nlbn.ValidateEnum("health check protocol", hcpRaw, nlbn.HealthCheckProtocols...)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	hc := &nlb.HealthCheckerDetails{Protocol: nlb.HealthCheckProtocolsEnum(hcp)}
	if v, ok, err := nlbn.OptionalInt("health_check_port", inputs); err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	} else if ok {
		hc.Port = &v
	}
	if v := strings.TrimSpace(nlbn.OptionalString("health_check_url_path", inputs)); v != "" {
		hc.UrlPath = &v
	}
	details := nlb.CreateBackendSetDetails{
		Name:          &name,
		Policy:        nlb.NetworkLoadBalancingPolicyEnum(policy),
		HealthChecker: hc,
	}
	if nlbn.BoolWasSet("is_preserve_source", inputs) {
		p := nlbn.OptionalBool("is_preserve_source", inputs, false)
		details.IsPreserveSource = &p
	}
	resp, err := client.CreateBackendSet(nlbn.Context(), nlb.CreateBackendSetRequest{
		NetworkLoadBalancerId:   &nlbID,
		CreateBackendSetDetails: details,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Creating backend set %q (%s) on network load balancer %s — poll work request %s", name, policy, nlbID, nlbn.Str(resp.OpcWorkRequestId)),
		"backend_set_name": name,
		"work_request_id":  nlbn.Str(resp.OpcWorkRequestId),
		"success":          true,
	}, nil
}
