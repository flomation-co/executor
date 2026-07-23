// Package oracle_loadbalancer_load_balancer_create provisions an OCI Load Balancer.
// Provisioning is asynchronous — OCI returns a work-request id which Get Work Request
// polls to completion; the new load balancer's OCID is then read with List/Get Load
// Balancer once the work request succeeds.
package oracle_loadbalancer_load_balancer_create

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
	Name         = "OCI Load Balancer: Create Load Balancer"
	Description  = "Provision an Oracle Cloud Layer-7 load balancer in a compartment across one or two subnets. Provisioning is asynchronous — returns a work-request id to poll with Get Work Request."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console", Required: true},
	{Name: "shape_name", Type: core.ConnectionTypeString, Label: "Shape", Placeholder: "flexible (recommended), or a fixed shape like 100Mbps / 400Mbps", Required: true},
	{Name: "subnet_ocids", Type: core.ConnectionTypeString, Label: "Subnet OCIDs", Placeholder: "One (regional) or two (AD-specific) subnet OCIDs, comma-separated", Required: true},
	{Name: "flex_min_mbps", Type: core.ConnectionTypeString, Label: "Flexible Min Bandwidth (Mbps)", Placeholder: "Required for the flexible shape, e.g. 10"},
	{Name: "flex_max_mbps", Type: core.ConnectionTypeString, Label: "Flexible Max Bandwidth (Mbps)", Placeholder: "Required for the flexible shape, e.g. 100"},
	{Name: "is_private", Type: core.ConnectionTypeBoolean, Label: "Private", Placeholder: "Create a private (internal) load balancer with no public IP (optional)"},
	{Name: "network_security_group_ocids", Type: core.ConnectionTypeString, Label: "Network Security Group OCIDs", Placeholder: "Comma-separated NSG OCIDs to attach (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lbn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	displayName, err := lbn.RequiredString("display_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	shape, err := lbn.RequiredString("shape_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	subnets := lbn.InputStrings("subnet_ocids", inputs)
	if len(subnets) == 0 {
		return lbn.ErrorResult("at least one subnet OCID is required"), nil
	}
	tags, err := lbn.FreeformTags("tags", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	details := lb.CreateLoadBalancerDetails{
		CompartmentId: &compartment,
		DisplayName:   &displayName,
		ShapeName:     &shape,
		SubnetIds:     subnets,
		FreeformTags:  tags,
	}
	if nsgs := lbn.InputStrings("network_security_group_ocids", inputs); len(nsgs) > 0 {
		details.NetworkSecurityGroupIds = nsgs
	}
	if lbn.BoolWasSet("is_private", inputs) {
		p := lbn.OptionalBool("is_private", inputs, false)
		details.IsPrivate = &p
	}
	// The flexible shape needs a bandwidth band; fixed shapes (100Mbps etc.) don't.
	minSet, maxSet := false, false
	var minMbps, maxMbps int
	if v, ok, err := lbn.OptionalInt("flex_min_mbps", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		minMbps, minSet = v, true
	}
	if v, ok, err := lbn.OptionalInt("flex_max_mbps", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		maxMbps, maxSet = v, true
	}
	if strings.EqualFold(shape, "flexible") {
		if !minSet || !maxSet {
			return lbn.ErrorResult("the flexible shape requires both a min and max bandwidth (Mbps)"), nil
		}
		details.ShapeDetails = &lb.ShapeDetails{MinimumBandwidthInMbps: &minMbps, MaximumBandwidthInMbps: &maxMbps}
	}

	resp, err := client.CreateLoadBalancer(lbn.Context(), lb.CreateLoadBalancerRequest{CreateLoadBalancerDetails: details})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return lbn.AsyncResult(fmt.Sprintf("Provisioning load balancer %q — poll work request %s", displayName, lbn.Str(resp.OpcWorkRequestId)), lbn.Str(resp.OpcWorkRequestId)), nil
}
