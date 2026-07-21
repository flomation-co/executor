// Package oracle_networking_route_table_get fetches one route table by OCID.
package oracle_networking_route_table_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Get Route table"
	Description  = "Fetch one Oracle Cloud route table by OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
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
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "route_table", Type: core.ConnectionTypeObject, Label: "Route Table"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "route_table_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetRouteTable(net.Context(), ocicore.GetRouteTableRequest{RtId: &id})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	rt := net.SummariseRouteTable(&resp.RouteTable)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Route table %q is %s", rt["display_name"], rt["lifecycle_state"]),
		"route_table":     rt,
		"lifecycle_state": rt["lifecycle_state"],
		"success":         true,
	}, nil
}
