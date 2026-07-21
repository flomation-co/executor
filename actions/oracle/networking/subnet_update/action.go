// Package oracle_networking_subnet_update updates the editable attributes of an
// Oracle Cloud subnet — its display name, associated route table / DHCP options /
// security lists, CIDR block and freeform tags.
package oracle_networking_subnet_update

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
	Name         = "OCI Networking: Update Subnet"
	Description  = "Update editable attributes of an Oracle Cloud subnet."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
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
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "cidr_block", Type: core.ConnectionTypeString, Label: "CIDR Block", Placeholder: "New CIDR, e.g. 10.0.1.0/24 — must stay within the VCN and cover the in-use range (optional)"},
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa… the subnet will use (optional)"},
	{Name: "dhcp_options_ocid", Type: core.ConnectionTypeString, Label: "DHCP Options OCID", Placeholder: "ocid1.dhcpoptions.oc1..aaaa… the subnet will use (optional)"},
	{Name: "security_list_ocids", Type: core.ConnectionTypeString, Label: "Security List OCIDs", Placeholder: "Comma-separated; replaces the entire current set (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces the current freeform tags (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subnet", Type: core.ConnectionTypeObject, Label: "Subnet"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "subnet_ocid")
	if errResult != nil {
		return errResult, nil
	}
	details := ocicore.UpdateSubnetDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("cidr_block", inputs)); v != "" {
		details.CidrBlock = &v
	}
	if v := strings.TrimSpace(net.OptionalString("route_table_ocid", inputs)); v != "" {
		details.RouteTableId = &v
	}
	if v := strings.TrimSpace(net.OptionalString("dhcp_options_ocid", inputs)); v != "" {
		details.DhcpOptionsId = &v
	}
	if ids := net.InputStrings("security_list_ocids", inputs); len(ids) > 0 {
		details.SecurityListIds = ids
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateSubnet(net.Context(), ocicore.UpdateSubnetRequest{SubnetId: &id, UpdateSubnetDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	subnet := net.SummariseSubnet(&resp.Subnet)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated subnet %q (%s)", subnet["display_name"], subnet["lifecycle_state"]),
		"subnet":          subnet,
		"lifecycle_state": subnet["lifecycle_state"],
		"success":         true,
	}, nil
}
