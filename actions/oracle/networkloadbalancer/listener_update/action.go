// Package oracle_networkloadbalancer_listener_update updates a listener (its default
// backend set, port or protocol) on a network load balancer. The NLB UpdateListener
// call is a FULL-REPLACE PUT, so this action first reads the current listener and
// seeds every field from it, overlaying only the values the operator supplied — a
// blank input keeps the current value. Asynchronous — returns a work-request id.
package oracle_networkloadbalancer_listener_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Update Listener"
	Description  = "Update a listener (its default backend set, port or protocol) on an Oracle Cloud network load balancer. A full-replace update: the current listener is read first and any field left blank keeps its current value. Asynchronous."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "listener_name", Type: core.ConnectionTypeString, Label: "Listener Name", Placeholder: "The listener to update, e.g. tcp-listener", Required: true},
	{Name: "default_backend_set_name", Type: core.ConnectionTypeString, Label: "Default Backend Set Name", Placeholder: "New backend set to forward to (blank = keep current)"},
	{Name: "port", Type: core.ConnectionTypeString, Label: "Port", Placeholder: "New listen port, e.g. 443 (blank = keep current)"},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "TCP, UDP, TCP_AND_UDP, L3IP or ANY (blank = keep current)"},
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

	// Read the current listener so the full-replace PUT does not null the fields the
	// operator left untouched (rule: read-modify-write to avoid data loss).
	getResp, err := client.GetListener(nlbn.Context(), nlb.GetListenerRequest{
		NetworkLoadBalancerId: &nlbID,
		ListenerName:          &name,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	cur := getResp.Listener

	// Seed every updatable field from the current listener.
	details := nlb.UpdateListenerDetails{
		DefaultBackendSetName: cur.DefaultBackendSetName,
		Port:                  cur.Port,
		Protocol:              cur.Protocol,
		IpVersion:             cur.IpVersion,
		IsPpv2Enabled:         cur.IsPpv2Enabled,
		TcpIdleTimeout:        cur.TcpIdleTimeout,
		UdpIdleTimeout:        cur.UdpIdleTimeout,
		L3IpIdleTimeout:       cur.L3IpIdleTimeout,
	}

	// Overlay only the values the operator supplied (blank = keep current).
	if bs := nlbn.OptionalString("default_backend_set_name", inputs); bs != "" {
		details.DefaultBackendSetName = &bs
	}
	if port, ok, perr := nlbn.OptionalInt("port", inputs); perr != nil {
		return nlbn.ErrorResult(perr.Error()), nil
	} else if ok {
		details.Port = &port
	}
	if protoRaw := nlbn.OptionalString("protocol", inputs); protoRaw != "" {
		proto, perr := nlbn.ValidateEnum("protocol", protoRaw, nlbn.ListenerProtocols...)
		if perr != nil {
			return nlbn.ErrorResult(perr.Error()), nil
		}
		details.Protocol = nlb.ListenerProtocolsEnum(proto)
	}

	resp, err := client.UpdateListener(nlbn.Context(), nlb.UpdateListenerRequest{
		NetworkLoadBalancerId: &nlbID,
		ListenerName:          &name,
		UpdateListenerDetails: details,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updating listener %q on network load balancer %s — poll work request %s", name, nlbID, nlbn.Str(resp.OpcWorkRequestId)),
		"listener_name":   name,
		"work_request_id": nlbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
