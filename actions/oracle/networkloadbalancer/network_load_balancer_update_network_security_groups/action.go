// Package oracle_networkloadbalancer_network_load_balancer_update_network_security_groups
// replaces the set of network security groups attached to a network load balancer.
// Asynchronous — returns a work-request id.
package oracle_networkloadbalancer_network_load_balancer_update_network_security_groups

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Update NSGs"
	Description  = "Replace the set of network security groups attached to an Oracle Cloud network load balancer. REPLACE semantics — the OCIDs you supply overwrite the current list; leaving the field empty removes all network security groups. Asynchronous — returns a work-request id to poll with Get Work Request."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+ethernet"
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
	{Name: "network_security_group_ocids", Type: core.ConnectionTypeString, Label: "Network Security Group OCIDs", Placeholder: "Comma-separated OCIDs. REPLACE semantics — this overwrites the current set; leave empty to remove all."},
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
	// REPLACE semantics: the supplied OCIDs overwrite the current set; an empty/nil
	// list removes all network security groups from the network load balancer.
	nsgIDs := nlbn.InputStrings("network_security_group_ocids", inputs)
	resp, err := client.UpdateNetworkSecurityGroups(nlbn.Context(), nlb.UpdateNetworkSecurityGroupsRequest{
		NetworkLoadBalancerId: &id,
		UpdateNetworkSecurityGroupsDetails: nlb.UpdateNetworkSecurityGroupsDetails{
			NetworkSecurityGroupIds: nsgIDs,
		},
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Network security groups updated (%d group(s)) for network load balancer %s — poll work request %s", len(nsgIDs), id, nlbn.Str(resp.OpcWorkRequestId)),
		"id":              id,
		"work_request_id": nlbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
