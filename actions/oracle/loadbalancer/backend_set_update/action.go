// Package oracle_loadbalancer_backend_set_update updates a backend set on a load
// balancer — replacing its load-balancing policy and health checker. Update is
// REPLACE-semantics: the details overwrite the whole backend set. Asynchronous —
// returns a work-request id.
package oracle_loadbalancer_backend_set_update

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
	Name         = "OCI Load Balancer: Update Backend Set"
	Description  = "Update a backend set on an Oracle Cloud load balancer — replace its load-balancing policy (e.g. ROUND_ROBIN) and health check. Replace-semantics: the details overwrite the whole backend set. Asynchronous."
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
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The name of the backend set to update, e.g. web-servers", Required: true},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Policy", Placeholder: "ROUND_ROBIN, LEAST_CONNECTIONS or IP_HASH — blank keeps the current value"},
	{Name: "health_check_protocol", Type: core.ConnectionTypeString, Label: "Health Check Protocol", Placeholder: "HTTP or TCP — blank keeps the current value"},
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
	// Replace-semantics: UpdateBackendSet is a full-replacement PUT — Backends is
	// mandatory and the whole Policy + HealthChecker are overwritten. So read the
	// current set first and seed everything from it, then overlay only what the
	// operator supplied. This keeps the backend membership (managed by the dedicated
	// Backend actions), and stops a policy-only change from nulling the tuned health
	// probe (e.g. an HTTP check needs its URL path, which has no dedicated input here).
	cur, err := client.GetBackendSet(lbn.Context(), lb.GetBackendSetRequest{LoadBalancerId: &lbID, BackendSetName: &name})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	policy := lbn.Str(cur.BackendSet.Policy)
	if v := strings.TrimSpace(lbn.OptionalString("policy", inputs)); v != "" {
		if policy, err = lbn.ValidateEnum("policy", v, lbn.BackendPolicies...); err != nil {
			return lbn.ErrorResult(err.Error()), nil
		}
	}
	hc := &lb.HealthCheckerDetails{}
	if h := cur.BackendSet.HealthChecker; h != nil {
		hc.Protocol = h.Protocol
		hc.Port = h.Port
		hc.UrlPath = h.UrlPath
		hc.ReturnCode = h.ReturnCode
		hc.Retries = h.Retries
		hc.TimeoutInMillis = h.TimeoutInMillis
		hc.IntervalInMillis = h.IntervalInMillis
		hc.ResponseBodyRegex = h.ResponseBodyRegex
		hc.IsForcePlainText = h.IsForcePlainText
	}
	if v := strings.TrimSpace(lbn.OptionalString("health_check_protocol", inputs)); v != "" {
		hcp, err := lbn.ValidateEnum("health check protocol", v, lbn.HealthCheckProtocols...)
		if err != nil {
			return lbn.ErrorResult(err.Error()), nil
		}
		hc.Protocol = &hcp
	}
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
	backends := make([]lb.BackendDetails, 0, len(cur.BackendSet.Backends))
	for i := range cur.BackendSet.Backends {
		b := cur.BackendSet.Backends[i]
		backends = append(backends, lb.BackendDetails{
			IpAddress: b.IpAddress, Port: b.Port, Weight: b.Weight,
			Backup: b.Backup, Drain: b.Drain, Offline: b.Offline,
		})
	}
	resp, err := client.UpdateBackendSet(lbn.Context(), lb.UpdateBackendSetRequest{
		LoadBalancerId: &lbID,
		BackendSetName: &name,
		UpdateBackendSetDetails: lb.UpdateBackendSetDetails{
			Policy:        &policy,
			HealthChecker: hc,
			Backends:      backends,
		},
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Updating backend set %q on load balancer %s — poll work request %s", name, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"backend_set_name": name,
		"work_request_id":  lbn.Str(resp.OpcWorkRequestId),
		"success":          true,
	}, nil
}
