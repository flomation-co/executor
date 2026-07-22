// Package oracle_networkloadbalancer_network_load_balancer_create provisions an OCI
// Network Load Balancer in a subnet. Provisioning is asynchronous, but the create call
// returns the new NLB's OCID immediately (in a CREATING state) alongside a
// work-request id to poll to completion.
package oracle_networkloadbalancer_network_load_balancer_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Create Network Load Balancer"
	Description  = "Provision an Oracle Cloud Layer 3/4 network load balancer in a subnet. Returns the new NLB's OCID immediately plus a work-request id to poll with Get Work Request."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… the NLB lives in", Required: true},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private", Placeholder: "Create a private (internal) NLB with no public IP (optional)"},
	{Name: "is_preserve_source_destination", Type: core.ConnectionTypeBoolean, Label: "Preserve Source/Destination", Placeholder: "Preserve the packet's source & destination (for transparent proxies) (optional)"},
	{Name: "network_security_group_ocids", Type: core.ConnectionTypeString, Label: "Network Security Group OCIDs", Placeholder: "Comma-separated NSG OCIDs to attach (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_load_balancer", Type: core.ConnectionTypeObject, Label: "Network Load Balancer"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := nlbn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	displayName, err := nlbn.RequiredString("display_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	subnet, err := nlbn.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	tags, err := nlbn.FreeformTags("tags", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	details := nlb.CreateNetworkLoadBalancerDetails{
		CompartmentId: &compartment,
		DisplayName:   &displayName,
		SubnetId:      &subnet,
		FreeformTags:  tags,
	}
	if nsgs := nlbn.InputStrings("network_security_group_ocids", inputs); len(nsgs) > 0 {
		details.NetworkSecurityGroupIds = nsgs
	}
	if nlbn.BoolWasSet("is_private", inputs) {
		p := nlbn.OptionalBool("is_private", inputs, false)
		details.IsPrivate = &p
	}
	if nlbn.BoolWasSet("is_preserve_source_destination", inputs) {
		p := nlbn.OptionalBool("is_preserve_source_destination", inputs, false)
		details.IsPreserveSourceDestination = &p
	}
	resp, err := client.CreateNetworkLoadBalancer(nlbn.Context(), nlb.CreateNetworkLoadBalancerRequest{CreateNetworkLoadBalancerDetails: details})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	summary := nlbn.SummariseNetworkLoadBalancer(&resp.NetworkLoadBalancer)
	return map[string]interface{}{
		"tool_result":           fmt.Sprintf("Provisioning network load balancer %q (%s) — poll work request %s", displayName, summary["lifecycle_state"], nlbn.Str(resp.OpcWorkRequestId)),
		"network_load_balancer": summary,
		"id":                    summary["id"],
		"lifecycle_state":       summary["lifecycle_state"],
		"work_request_id":       nlbn.Str(resp.OpcWorkRequestId),
		"success":               true,
	}, nil
}
