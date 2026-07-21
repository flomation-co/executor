// Package oracle_networking_service_gateway_update edits the mutable attributes of
// a service gateway — its display name, block-traffic switch, associated route
// table and freeform tags.
package oracle_networking_service_gateway_update

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
	Name         = "OCI Networking: Update Service gateway"
	Description  = "Update editable attributes of an Oracle Cloud service gateway."
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
	{Name: "service_gateway_ocid", Type: core.ConnectionTypeString, Label: "Service Gateway OCID", Placeholder: "ocid1.servicegateway.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "block_traffic", Type: core.ConnectionTypeBoolean, Label: "Block Traffic", Placeholder: "Block all traffic through the gateway without deleting it (optional)"},
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa… — route table for the gateway (optional)"},
	{Name: "service_ocids", Type: core.ConnectionTypeString, Label: "Service OCIDs", Placeholder: "Comma-separated Service OCIDs — replaces the enabled services when supplied (see List Services)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "service_gateway", Type: core.ConnectionTypeObject, Label: "Service Gateway"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "service_gateway_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateServiceGatewayDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if net.BoolWasSet("block_traffic", inputs) {
		block := net.OptionalBool("block_traffic", inputs, false)
		details.BlockTraffic = &block
	}
	if v := strings.TrimSpace(net.OptionalString("route_table_ocid", inputs)); v != "" {
		details.RouteTableId = &v
	}
	// REPLACE semantics: the enabled services are only changed when the operator
	// supplies OCIDs (an empty input preserves the current service list).
	if svc := net.InputStrings("service_ocids", inputs); len(svc) > 0 {
		services := make([]ocicore.ServiceIdRequestDetails, 0, len(svc))
		for _, s := range svc {
			sid := s
			services = append(services, ocicore.ServiceIdRequestDetails{ServiceId: &sid})
		}
		details.Services = services
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateServiceGateway(net.Context(), ocicore.UpdateServiceGatewayRequest{ServiceGatewayId: &id, UpdateServiceGatewayDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	gateway := net.SummariseServiceGateway(&resp.ServiceGateway)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated service gateway %q (%s)", gateway["display_name"], gateway["lifecycle_state"]),
		"service_gateway": gateway,
		"lifecycle_state": gateway["lifecycle_state"],
		"success":         true,
	}, nil
}
