// Package oracle_networkloadbalancer_listener_create adds a listener (a protocol+port
// the NLB accepts traffic on, forwarding to a default backend set) to a network load
// balancer. Asynchronous — returns a work-request id.
package oracle_networkloadbalancer_listener_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Create Listener"
	Description  = "Add a listener — a protocol and port the Oracle Cloud network load balancer accepts traffic on (TCP, UDP, TCP_AND_UDP, L3IP or ANY) — forwarding to a default backend set. Asynchronous."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
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
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name", Placeholder: "A name unique within the NLB, e.g. tcp-listener", Required: true},
	{Name: "default_backend_set_name", Type: core.ConnectionTypeString, Label: "Default Backend Set Name", Placeholder: "The backend set to forward to, e.g. app-servers", Required: true},
	{Name: "port", Type: core.ConnectionTypeString, Label: "Port", Placeholder: "The port to listen on, e.g. 443 (0 = all ports, for L3IP)", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "TCP, UDP, TCP_AND_UDP, L3IP or ANY", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, nlbID, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := nlbn.RequiredString("listener_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	backendSet, err := nlbn.RequiredString("default_backend_set_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	port, err := nlbn.RequiredInt("port", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	protoRaw, err := nlbn.RequiredString("protocol", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	proto, err := nlbn.ValidateEnum("protocol", protoRaw, nlbn.ListenerProtocols...)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateListener(nlbn.Context(), nlb.CreateListenerRequest{
		NetworkLoadBalancerId: &nlbID,
		CreateListenerDetails: nlb.CreateListenerDetails{
			Name:                  &name,
			DefaultBackendSetName: &backendSet,
			Port:                  &port,
			Protocol:              nlb.ListenerProtocolsEnum(proto),
		},
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Creating listener %q (%s:%d) on network load balancer %s — poll work request %s", name, proto, port, nlbID, nlbn.Str(resp.OpcWorkRequestId)),
		"listener_name":   name,
		"work_request_id": nlbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
