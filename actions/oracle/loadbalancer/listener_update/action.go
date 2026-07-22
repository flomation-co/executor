// Package oracle_loadbalancer_listener_update replaces a listener's configuration
// (default backend set, port, protocol, hostnames, path-route/routing-policy) on a
// load balancer. Asynchronous — returns a work-request id.
package oracle_loadbalancer_listener_update

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
	Name         = "OCI Load Balancer: Update Listener"
	Description  = "Update a listener on an Oracle Cloud load balancer — its default backend set, port, protocol, hostnames and path-route/routing-policy. Replaces the whole listener configuration. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
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
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name", Placeholder: "The name of the listener to update, e.g. http-listener", Required: true},
	{Name: "default_backend_set_name", Type: core.ConnectionTypeString, Label: "Default Backend Set Name", Placeholder: "The backend set to forward to, e.g. web-servers", Required: true},
	{Name: "port", Type: core.ConnectionTypeString, Label: "Port", Placeholder: "The port to listen on, e.g. 80 or 443", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "HTTP, HTTP2, TCP or GRPC", Required: true},
	{Name: "hostname_names", Type: core.ConnectionTypeString, Label: "Hostname Names", Placeholder: "Comma-separated virtual-hostname names to match (optional)"},
	{Name: "path_route_set_name", Type: core.ConnectionTypeString, Label: "Path Route Set Name", Placeholder: "A path-route set to apply for URL routing (optional)"},
	{Name: "routing_policy_name", Type: core.ConnectionTypeString, Label: "Routing Policy Name", Placeholder: "A routing policy to apply (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := lbn.RequiredString("listener_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	backendSet, err := lbn.RequiredString("default_backend_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	port, err := lbn.RequiredInt("port", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	protocol, err := lbn.RequiredString("protocol", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// Replace-semantics: UpdateListenerDetails overwrites the whole listener, so build
	// it from the operator's inputs exactly as listener_create builds CreateListenerDetails.
	details := lb.UpdateListenerDetails{
		DefaultBackendSetName: &backendSet,
		Port:                  &port,
		Protocol:              &protocol,
	}
	if h := lbn.InputStrings("hostname_names", inputs); len(h) > 0 {
		details.HostnameNames = h
	}
	if v := strings.TrimSpace(lbn.OptionalString("path_route_set_name", inputs)); v != "" {
		details.PathRouteSetName = &v
	}
	if v := strings.TrimSpace(lbn.OptionalString("routing_policy_name", inputs)); v != "" {
		details.RoutingPolicyName = &v
	}
	// Replace-semantics: UpdateListener is a full-replace PUT, so config this node
	// doesn't expose (SSL termination, connection config, rule sets) would be dropped —
	// e.g. re-pointing an HTTPS listener's backend set would silently strip its TLS.
	// Read the current listener (the classic LB API has no standalone GetListener, so
	// read it off the parent) and carry those fields forward.
	lbResp, err := client.GetLoadBalancer(lbn.Context(), lb.GetLoadBalancerRequest{LoadBalancerId: &lbID})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	if cur, ok := lbResp.LoadBalancer.Listeners[name]; ok {
		details.ConnectionConfiguration = cur.ConnectionConfiguration
		if len(cur.RuleSetNames) > 0 {
			details.RuleSetNames = cur.RuleSetNames
		}
		// Carry forward the association fields the operator didn't override, so
		// changing only the port doesn't drop an existing hostname / path-route /
		// routing-policy binding.
		if details.HostnameNames == nil {
			details.HostnameNames = cur.HostnameNames
		}
		if details.PathRouteSetName == nil {
			details.PathRouteSetName = cur.PathRouteSetName
		}
		if details.RoutingPolicyName == nil {
			details.RoutingPolicyName = cur.RoutingPolicyName
		}
		if s := cur.SslConfiguration; s != nil {
			details.SslConfiguration = &lb.SslConfigurationDetails{
				VerifyDepth:                    s.VerifyDepth,
				VerifyPeerCertificate:          s.VerifyPeerCertificate,
				HasSessionResumption:           s.HasSessionResumption,
				TrustedCertificateAuthorityIds: s.TrustedCertificateAuthorityIds,
				CertificateIds:                 s.CertificateIds,
				CertificateName:                s.CertificateName,
				Protocols:                      s.Protocols,
				CipherSuiteName:                s.CipherSuiteName,
				ServerOrderPreference:          lb.SslConfigurationDetailsServerOrderPreferenceEnum(s.ServerOrderPreference),
			}
		}
	}
	resp, err := client.UpdateListener(lbn.Context(), lb.UpdateListenerRequest{
		LoadBalancerId:        &lbID,
		ListenerName:          &name,
		UpdateListenerDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updating listener %q (%s:%d) on load balancer %s — poll work request %s", name, protocol, port, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"listener_name":   name,
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
