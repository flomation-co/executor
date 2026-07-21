// Package oracle_networking_nat_gateway_get fetches one NAT gateway by OCID.
package oracle_networking_nat_gateway_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Get NAT Gateway"
	Description  = "Fetch one Oracle Cloud NAT gateway by OCID."
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
	{Name: "nat_gateway_ocid", Type: core.ConnectionTypeString, Label: "NAT Gateway OCID", Placeholder: "ocid1.natgateway.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nat_gateway", Type: core.ConnectionTypeObject, Label: "NAT Gateway"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "nat_gateway_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetNatGateway(net.Context(), ocicore.GetNatGatewayRequest{NatGatewayId: &id})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	natGateway := net.SummariseNatGateway(&resp.NatGateway)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("NAT gateway %q is %s", natGateway["display_name"], natGateway["lifecycle_state"]),
		"nat_gateway":     natGateway,
		"lifecycle_state": natGateway["lifecycle_state"],
		"success":         true,
	}, nil
}
