// Package oracle_networkloadbalancer_network_load_balancer_update updates a network
// load balancer's mutable top-level fields. Every field on the update details struct
// is optional, so this is a safe partial update: only the fields the operator supplies
// are sent, leaving the rest untouched. Asynchronous — returns a work-request id.
package oracle_networkloadbalancer_network_load_balancer_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Update Network Load Balancer"
	Description  = "Update an Oracle Cloud network load balancer's mutable top-level fields — its display name and the source/destination preservation and symmetric-hash flags. Only the fields you supply are changed. Asynchronous — returns a work-request id to poll with Get Work Request."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+ethernet"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New display name (leave blank to keep current)"},
	{Name: "is_preserve_source_destination", Type: core.ConnectionTypeBoolean, Label: "Preserve Source/Destination", Placeholder: "Send the full IP header to backends (leave blank to keep current)"},
	{Name: "is_symmetric_hash_enabled", Type: core.ConnectionTypeBoolean, Label: "Symmetric Hash Enabled", Placeholder: "Only valid in transparent mode with source/destination preservation (leave blank to keep current)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}

	var details nlb.UpdateNetworkLoadBalancerDetails
	changed := false

	if name := nlbn.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
		changed = true
	}
	if nlbn.BoolWasSet("is_preserve_source_destination", inputs) {
		v := nlbn.OptionalBool("is_preserve_source_destination", inputs, false)
		details.IsPreserveSourceDestination = &v
		changed = true
	}
	if nlbn.BoolWasSet("is_symmetric_hash_enabled", inputs) {
		v := nlbn.OptionalBool("is_symmetric_hash_enabled", inputs, false)
		details.IsSymmetricHashEnabled = &v
		changed = true
	}

	if !changed {
		return nlbn.ErrorResult("nothing to update — supply at least one of display name, preserve source/destination, or symmetric hash enabled"), nil
	}

	resp, err := client.UpdateNetworkLoadBalancer(nlbn.Context(), nlb.UpdateNetworkLoadBalancerRequest{
		NetworkLoadBalancerId:            &id,
		UpdateNetworkLoadBalancerDetails: details,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Update requested for network load balancer %s — poll work request %s", id, nlbn.Str(resp.OpcWorkRequestId)),
		"id":              id,
		"work_request_id": nlbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
