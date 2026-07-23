// Package oracle_networking_subnet_create creates a subnet in a VCN — a CIDR range
// that VNICs and instances attach to. Omit the availability domain for a regional subnet.
package oracle_networking_subnet_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Create Subnet"
	Description  = "Create a subnet in a VCN — a CIDR range that VNICs and instances attach to. Omit the availability domain for a regional subnet."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
	Date         = "21/07/2026"
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
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID", Placeholder: "ocid1.vcn.oc1..aaaa…", Required: true},
	{Name: "cidr_block", Type: core.ConnectionTypeString, Label: "CIDR Block", Placeholder: "e.g. 10.0.1.0/24 (must fall within a VCN CIDR)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (omit for a regional subnet)"},
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa… (defaults to the VCN's default route table)"},
	{Name: "security_list_ocids", Type: core.ConnectionTypeString, Label: "Security List OCIDs", Placeholder: "Comma-separated (defaults to the VCN's default security list)"},
	{Name: "dhcp_options_ocid", Type: core.ConnectionTypeString, Label: "DHCP Options OCID", Placeholder: "ocid1.dhcpoptions.oc1..aaaa… (defaults to the VCN's default set)"},
	{Name: "dns_label", Type: core.ConnectionTypeString, Label: "DNS Label", Placeholder: "Alphanumeric DNS label, e.g. subnet123 (optional, immutable once set)"},
	{Name: "prohibit_public_ip_on_vnic", Type: core.ConnectionTypeBoolean, Label: "Prohibit Public IP on VNIC", Placeholder: "true makes this a private subnet (no public IPs)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subnet", Type: core.ConnectionTypeObject, Label: "Subnet"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subnet OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := net.GetAuth(inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	vcnID, err := net.RequiredString("vcn_ocid", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	cidr, err := net.RequiredString("cidr_block", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreateSubnetDetails{
		CompartmentId: &compartment,
		VcnId:         &vcnID,
		CidrBlock:     &cidr,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("availability_domain", inputs)); v != "" {
		details.AvailabilityDomain = &v
	}
	if v := strings.TrimSpace(net.OptionalString("route_table_ocid", inputs)); v != "" {
		details.RouteTableId = &v
	}
	if ids := net.InputStrings("security_list_ocids", inputs); len(ids) > 0 {
		details.SecurityListIds = ids
	}
	if v := strings.TrimSpace(net.OptionalString("dhcp_options_ocid", inputs)); v != "" {
		details.DhcpOptionsId = &v
	}
	if v := strings.TrimSpace(net.OptionalString("dns_label", inputs)); v != "" {
		details.DnsLabel = &v
	}
	if net.BoolWasSet("prohibit_public_ip_on_vnic", inputs) {
		prohibit := net.OptionalBool("prohibit_public_ip_on_vnic", inputs, false)
		details.ProhibitPublicIpOnVnic = &prohibit
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateSubnet(net.Context(), ocicore.CreateSubnetRequest{CreateSubnetDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	subnet := net.SummariseSubnet(&resp.Subnet)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created subnet %q (%s)", subnet["display_name"], subnet["lifecycle_state"]),
		"subnet":          subnet,
		"id":              subnet["id"],
		"lifecycle_state": subnet["lifecycle_state"],
		"success":         true,
	}, nil
}
