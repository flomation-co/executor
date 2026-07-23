// Package oracle_loadbalancer_backend_set_update_health_checker replaces the
// health-checker policy of a backend set on an Oracle Cloud load balancer.
// Asynchronous — returns a work-request id.
package oracle_loadbalancer_backend_set_update_health_checker

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
	Name         = "OCI Load Balancer: Update Health Checker"
	Description  = "Replace the health-checker policy (protocol, port, URL path, expected return code, retries and timing) of a backend set on an Oracle Cloud load balancer. Asynchronous."
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
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set whose health check to update, e.g. web-servers", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "HTTP or TCP — leave blank to keep the current value (optional)"},
	{Name: "port", Type: core.ConnectionTypeString, Label: "Port", Placeholder: "e.g. 80 (0 = use each backend's port)"},
	{Name: "url_path", Type: core.ConnectionTypeString, Label: "URL Path", Placeholder: "For HTTP checks, e.g. /health"},
	{Name: "return_code", Type: core.ConnectionTypeString, Label: "Return Code", Placeholder: "Expected HTTP status, e.g. 200"},
	{Name: "retries", Type: core.ConnectionTypeString, Label: "Retries", Placeholder: "Attempts before a backend is marked unhealthy, e.g. 3"},
	{Name: "interval_in_millis", Type: core.ConnectionTypeString, Label: "Interval (ms)", Placeholder: "Interval between health checks in ms, e.g. 10000"},
	{Name: "timeout_in_millis", Type: core.ConnectionTypeString, Label: "Timeout (ms)", Placeholder: "Max wait for a reply in ms, e.g. 3000"},
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
	// Replace-semantics: UpdateHealthCheckerDetails overwrites the backend set's whole
	// health-checker policy and marks Port/ReturnCode/Retries/Interval/Timeout/
	// ResponseBodyRegex all mandatory — sending nils would 400 or wipe the tuned probe
	// config. So read the current health checker first and seed every field from it,
	// then overlay only what the operator supplied (ResponseBodyRegex has no input, so
	// it always carries forward).
	cur, err := client.GetHealthChecker(lbn.Context(), lb.GetHealthCheckerRequest{LoadBalancerId: &lbID, BackendSetName: &name})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	details := lb.UpdateHealthCheckerDetails{
		Protocol:          cur.HealthChecker.Protocol,
		Port:              cur.HealthChecker.Port,
		ReturnCode:        cur.HealthChecker.ReturnCode,
		Retries:           cur.HealthChecker.Retries,
		TimeoutInMillis:   cur.HealthChecker.TimeoutInMillis,
		IntervalInMillis:  cur.HealthChecker.IntervalInMillis,
		ResponseBodyRegex: cur.HealthChecker.ResponseBodyRegex,
		UrlPath:           cur.HealthChecker.UrlPath,
		IsForcePlainText:  cur.HealthChecker.IsForcePlainText,
	}
	if v := strings.TrimSpace(lbn.OptionalString("protocol", inputs)); v != "" {
		p, err := lbn.ValidateEnum("protocol", v, lbn.HealthCheckProtocols...)
		if err != nil {
			return lbn.ErrorResult(err.Error()), nil
		}
		details.Protocol = &p
	}
	if v, ok, err := lbn.OptionalInt("port", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Port = &v
	}
	if v := strings.TrimSpace(lbn.OptionalString("url_path", inputs)); v != "" {
		details.UrlPath = &v
	}
	if v, ok, err := lbn.OptionalInt("return_code", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.ReturnCode = &v
	}
	if v, ok, err := lbn.OptionalInt("retries", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Retries = &v
	}
	if v, ok, err := lbn.OptionalInt("interval_in_millis", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.IntervalInMillis = &v
	}
	if v, ok, err := lbn.OptionalInt("timeout_in_millis", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.TimeoutInMillis = &v
	}
	resp, err := client.UpdateHealthChecker(lbn.Context(), lb.UpdateHealthCheckerRequest{
		LoadBalancerId:             &lbID,
		BackendSetName:             &name,
		UpdateHealthCheckerDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Updating health checker for backend set %q on load balancer %s — poll work request %s", name, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"backend_set_name": name,
		"work_request_id":  lbn.Str(resp.OpcWorkRequestId),
		"success":          true,
	}, nil
}
