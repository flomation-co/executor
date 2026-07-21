// Package oracle_networking_internet_gateway_update edits the mutable attributes
// of an internet gateway — its display name, enabled state, route table and tags.
package oracle_networking_internet_gateway_update

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
	Name         = "OCI Networking: Update Internet gateway"
	Description  = "Update editable attributes of an Oracle Cloud internet gateway."
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
	{Name: "internet_gateway_ocid", Type: core.ConnectionTypeString, Label: "Internet Gateway OCID", Placeholder: "ocid1.internetgateway.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Whether the gateway is enabled (optional — leave unset to keep current)"},
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa… the gateway should use (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
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
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateInternetGatewayDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if net.BoolWasSet("is_enabled", inputs) {
		enabled := net.OptionalBool("is_enabled", inputs, false)
		details.IsEnabled = &enabled
	}
	if v := strings.TrimSpace(net.OptionalString("route_table_ocid", inputs)); v != "" {
		details.RouteTableId = &v
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateInternetGateway(net.Context(), ocicore.UpdateInternetGatewayRequest{IgId: &id, UpdateInternetGatewayDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	ig := net.SummariseInternetGateway(&resp.InternetGateway)
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Updated internet gateway %q (%s)", ig["display_name"], ig["lifecycle_state"]),
		"internet_gateway": ig,
		"lifecycle_state":  ig["lifecycle_state"],
		"success":          true,
	}, nil
}
