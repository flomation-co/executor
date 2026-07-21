// Package oracle_networking_dhcp_options_delete deletes a set of DHCP options by
// OCID. The DHCP options must not be attached to any subnet as its active set.
package oracle_networking_dhcp_options_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Delete DHCP Options"
	Description  = "Delete an Oracle Cloud DHCP options by OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gear"
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
	{Name: "dhcp_options_ocid", Type: core.ConnectionTypeString, Label: "DHCP Options OCID", Placeholder: "ocid1.dhcpoptions.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "dhcp_options_ocid")
	if errResult != nil {
		return errResult, nil
	}
	_, err := client.DeleteDhcpOptions(net.Context(), ocicore.DeleteDhcpOptionsRequest{DhcpId: &id})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted DHCP options %s", id),
		"success":     true,
	}, nil
}
