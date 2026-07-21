// Package oracle_networking_nat_gateway_create creates a NAT gateway —
// outbound-only internet access for a VCN's private subnets.
package oracle_networking_nat_gateway_create

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
	Name         = "OCI Networking: Create NAT Gateway"
	Description  = "Create a NAT gateway in a VCN — outbound internet access for private subnets, with no inbound. Optionally pin a reserved public IP."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+ethernet"
	Date         = "21/07/2026"
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
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID", Placeholder: "ocid1.vcn.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "block_traffic", Type: core.ConnectionTypeBoolean, Label: "Block Traffic", Placeholder: "Whether the gateway blocks traffic through it (off by default)"},
	{Name: "public_ip_ocid", Type: core.ConnectionTypeString, Label: "Reserved Public IP OCID", Placeholder: "ocid1.publicip.oc1..aaaa… — pin a reserved public IP (optional)"},
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa… — route table for the gateway (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nat_gateway", Type: core.ConnectionTypeObject, Label: "NAT Gateway"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "NAT Gateway OCID"},
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
	details := ocicore.CreateNatGatewayDetails{
		CompartmentId: &compartment,
		VcnId:         &vcnID,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if net.BoolWasSet("block_traffic", inputs) {
		block := net.OptionalBool("block_traffic", inputs, false)
		details.BlockTraffic = &block
	}
	if v := strings.TrimSpace(net.OptionalString("public_ip_ocid", inputs)); v != "" {
		details.PublicIpId = &v
	}
	if v := strings.TrimSpace(net.OptionalString("route_table_ocid", inputs)); v != "" {
		details.RouteTableId = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateNatGateway(net.Context(), ocicore.CreateNatGatewayRequest{CreateNatGatewayDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	ng := net.SummariseNatGateway(&resp.NatGateway)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created NAT gateway %q (%s)", ng["display_name"], ng["lifecycle_state"]),
		"nat_gateway":     ng,
		"id":              ng["id"],
		"lifecycle_state": ng["lifecycle_state"],
		"success":         true,
	}, nil
}
