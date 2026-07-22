// Package oracle_loadbalancer_listener_create adds a listener (a protocol+port the
// load balancer accepts traffic on, forwarding to a default backend set) to a load
// balancer. Asynchronous — returns a work-request id.
package oracle_loadbalancer_listener_create

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
	Name         = "OCI Load Balancer: Create Listener"
	Description  = "Add a listener — a protocol and port the Oracle Cloud load balancer accepts traffic on — forwarding to a default backend set. Asynchronous — returns a work-request id."
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
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name", Placeholder: "A name unique within the load balancer, e.g. http-listener", Required: true},
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
	details := lb.CreateListenerDetails{
		Name:                  &name,
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
	resp, err := client.CreateListener(lbn.Context(), lb.CreateListenerRequest{
		LoadBalancerId:        &lbID,
		CreateListenerDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Creating listener %q (%s:%d) on load balancer %s — poll work request %s", name, protocol, port, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"listener_name":   name,
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
