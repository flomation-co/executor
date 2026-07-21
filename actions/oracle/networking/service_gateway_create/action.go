// Package oracle_networking_service_gateway_create creates a service gateway — private
// access to Oracle services (Object Storage, etc.) from a VCN without the public internet.
package oracle_networking_service_gateway_create

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
	Name         = "OCI Networking: Create Service Gateway"
	Description  = "Create a service gateway in a VCN — private access to Oracle services (Object Storage, etc.) without the public internet. Supply the target service OCIDs (see List Services)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plug"
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
	{Name: "service_ocids", Type: core.ConnectionTypeString, Label: "Service OCIDs", Placeholder: "Comma-separated Service object OCIDs to enable (see List Services)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa… the gateway will use (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "service_gateway", Type: core.ConnectionTypeObject, Label: "Service Gateway"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Service Gateway OCID"},
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
	serviceOCIDs := net.InputStrings("service_ocids", inputs)
	if len(serviceOCIDs) == 0 {
		return net.ErrorResult("at least one service OCID is required (see List Services)"), nil
	}
	services := make([]ocicore.ServiceIdRequestDetails, 0, len(serviceOCIDs))
	for _, s := range serviceOCIDs {
		id := s
		services = append(services, ocicore.ServiceIdRequestDetails{ServiceId: &id})
	}
	details := ocicore.CreateServiceGatewayDetails{
		CompartmentId: &compartment,
		VcnId:         &vcnID,
		Services:      services,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("route_table_ocid", inputs)); v != "" {
		details.RouteTableId = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateServiceGateway(net.Context(), ocicore.CreateServiceGatewayRequest{CreateServiceGatewayDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	sg := net.SummariseServiceGateway(&resp.ServiceGateway)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created service gateway %q (%s)", sg["display_name"], sg["lifecycle_state"]),
		"service_gateway": sg,
		"id":              sg["id"],
		"lifecycle_state": sg["lifecycle_state"],
		"success":         true,
	}, nil
}
