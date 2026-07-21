// Package oracle_networking_internet_gateway_get fetches one internet gateway by OCID.
package oracle_networking_internet_gateway_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Get Internet gateway"
	Description  = "Fetch one Oracle Cloud internet gateway by OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+globe"
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
	{Name: "internet_gateway_ocid", Type: core.ConnectionTypeString, Label: "Internet Gateway OCID", Placeholder: "ocid1.internetgateway.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "internet_gateway", Type: core.ConnectionTypeObject, Label: "Internet Gateway"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "internet_gateway_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetInternetGateway(net.Context(), ocicore.GetInternetGatewayRequest{IgId: &id})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	ig := net.SummariseInternetGateway(&resp.InternetGateway)
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Internet gateway %q is %s", ig["display_name"], ig["lifecycle_state"]),
		"internet_gateway": ig,
		"lifecycle_state":  ig["lifecycle_state"],
		"success":          true,
	}, nil
}
