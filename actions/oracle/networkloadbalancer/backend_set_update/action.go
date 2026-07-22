// Package oracle_networkloadbalancer_backend_set_update updates a backend set (its
// tuple-hash policy, health checker and preserve-source setting) on a network load
// balancer. Because the underlying PUT is a full replace, it GETs the current backend
// set first and seeds every field — carrying the existing backends and health checker
// forward untouched — before overlaying only what the operator supplied, so nothing is
// silently wiped. Asynchronous — returns a work-request id.
package oracle_networkloadbalancer_backend_set_update

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
	Name         = "OCI Network Load Balancer: Update Backend Set"
	Description  = "Update a backend set on an Oracle Cloud network load balancer — its tuple-hash policy, health checker and preserve-source setting. The current backends and any unspecified fields are read first and carried forward untouched (full-replace-safe). Blank fields keep their current value. Asynchronous."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The name of the backend set to update", Required: true},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Policy", Placeholder: "TWO_TUPLE, THREE_TUPLE or FIVE_TUPLE (blank = keep current)"},
	{Name: "health_check_protocol", Type: core.ConnectionTypeString, Label: "Health Check Protocol", Placeholder: "TCP, HTTP, HTTPS, UDP or DNS (blank = keep current)"},
	{Name: "health_check_port", Type: core.ConnectionTypeString, Label: "Health Check Port", Placeholder: "e.g. 80 (0 = use each backend's port; blank = keep current)"},
	{Name: "health_check_url_path", Type: core.ConnectionTypeString, Label: "Health Check URL Path", Placeholder: "For HTTP/HTTPS checks, e.g. /health (blank = keep current)"},
	{Name: "is_preserve_source", Type: core.ConnectionTypeBoolean, Label: "Preserve Source IP", Placeholder: "Pass the client's source IP to the backend (blank = keep current)"},
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

	// Read-modify-write: the UpdateBackendSet PUT is a FULL REPLACE, so fetch the
	// current backend set and seed every field from it before overlaying inputs.
	cur, err := client.GetBackendSet(nlbn.Context(), nlb.GetBackendSetRequest{
		NetworkLoadBalancerId: &nlbID,
		BackendSetName:        &name,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}

	details := seedFromCurrent(cur.BackendSet)

	// Overlay: policy (optional; blank = keep current).
	if raw := strings.TrimSpace(nlbn.OptionalString("policy", inputs)); raw != "" {
		policy, err := nlbn.ValidateEnum("policy", raw, nlbn.NlbPolicies...)
		if err != nil {
			return nlbn.ErrorResult(err.Error()), nil
		}
		details.Policy = &policy
	}

	// Overlay: preserve-source (optional; blank = keep current).
	if nlbn.BoolWasSet("is_preserve_source", inputs) {
		p := nlbn.OptionalBool("is_preserve_source", inputs, false)
		details.IsPreserveSource = &p
	}

	// Overlay: health checker fields (optional; blank = keep current). The seeded
	// health checker is always non-nil, but guard defensively.
	if details.HealthChecker == nil {
		details.HealthChecker = &nlb.HealthCheckerDetails{}
	}
	if raw := strings.TrimSpace(nlbn.OptionalString("health_check_protocol", inputs)); raw != "" {
		hcp, err := nlbn.ValidateEnum("health check protocol", raw, nlbn.HealthCheckProtocols...)
		if err != nil {
			return nlbn.ErrorResult(err.Error()), nil
		}
		details.HealthChecker.Protocol = nlb.HealthCheckProtocolsEnum(hcp)
	}
	if v, ok, err := nlbn.OptionalInt("health_check_port", inputs); err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.HealthChecker.Port = &v
	}
	if v := strings.TrimSpace(nlbn.OptionalString("health_check_url_path", inputs)); v != "" {
		details.HealthChecker.UrlPath = &v
	}

	resp, err := client.UpdateBackendSet(nlbn.Context(), nlb.UpdateBackendSetRequest{
		NetworkLoadBalancerId:   &nlbID,
		BackendSetName:          &name,
		UpdateBackendSetDetails: details,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Updating backend set %q on network load balancer %s — poll work request %s", name, nlbID, nlbn.Str(resp.OpcWorkRequestId)),
		"backend_set_name": name,
		"work_request_id":  nlbn.Str(resp.OpcWorkRequestId),
		"success":          true,
	}, nil
}

// seedFromCurrent builds a full UpdateBackendSetDetails from the backend set as it is
// today, so a full-replace PUT preserves everything the operator did not change.
func seedFromCurrent(bs nlb.BackendSet) nlb.UpdateBackendSetDetails {
	d := nlb.UpdateBackendSetDetails{
		IsPreserveSource:                        bs.IsPreserveSource,
		IsFailOpen:                              bs.IsFailOpen,
		IsInstantFailoverEnabled:                bs.IsInstantFailoverEnabled,
		IsInstantFailoverTcpResetEnabled:        bs.IsInstantFailoverTcpResetEnabled,
		AreOperationallyActiveBackendsPreferred: bs.AreOperationallyActiveBackendsPreferred,
		IpVersion:                               bs.IpVersion,
		Backends:                                seedBackends(bs.Backends),
		HealthChecker:                           seedHealthChecker(bs.HealthChecker),
	}
	// Policy is a *string on the update details (a typed enum on read) — carry the
	// current value across only when it is set.
	if p := string(bs.Policy); p != "" {
		d.Policy = &p
	}
	return d
}

// seedBackends converts the read-model backends into their details form so they ride
// along in the full-replace PUT untouched, carrying weight / backup / drain / offline.
func seedBackends(backends []nlb.Backend) []nlb.BackendDetails {
	if backends == nil {
		return nil
	}
	out := make([]nlb.BackendDetails, 0, len(backends))
	for i := range backends {
		b := backends[i]
		out = append(out, nlb.BackendDetails{
			Port:      b.Port,
			Name:      b.Name,
			IpAddress: b.IpAddress,
			TargetId:  b.TargetId,
			Weight:    b.Weight,
			IsBackup:  b.IsBackup,
			IsDrain:   b.IsDrain,
			IsOffline: b.IsOffline,
		})
	}
	return out
}

// seedHealthChecker converts the read-model health checker into its details form,
// carrying ALL fields (protocol, port, retries, timeouts, url path, return code, the
// request/response probe data and DNS block) so none are wiped by the full-replace PUT.
func seedHealthChecker(h *nlb.HealthChecker) *nlb.HealthCheckerDetails {
	if h == nil {
		return nil
	}
	return &nlb.HealthCheckerDetails{
		Protocol:          h.Protocol,
		Port:              h.Port,
		Retries:           h.Retries,
		TimeoutInMillis:   h.TimeoutInMillis,
		IntervalInMillis:  h.IntervalInMillis,
		UrlPath:           h.UrlPath,
		ResponseBodyRegex: h.ResponseBodyRegex,
		ReturnCode:        h.ReturnCode,
		RequestData:       h.RequestData,
		ResponseData:      h.ResponseData,
		Dns:               h.Dns,
	}
}
