// Package oracle_networking_vcn_get fetches one Virtual Cloud Network by OCID.
package oracle_networking_vcn_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Get VCN"
	Description  = "Fetch one Oracle Cloud Virtual Cloud Network (VCN) by OCID — its CIDR blocks, lifecycle state, DNS label and default route table / security list / DHCP options."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
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
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID", Placeholder: "ocid1.vcn.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vcn", Type: core.ConnectionTypeObject, Label: "VCN"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "vcn_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetVcn(net.Context(), ocicore.GetVcnRequest{VcnId: &id})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	vcn := net.SummariseVcn(&resp.Vcn)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("VCN %q is %s", vcn["display_name"], vcn["lifecycle_state"]),
		"vcn":             vcn,
		"lifecycle_state": vcn["lifecycle_state"],
		"success":         true,
	}, nil
}
