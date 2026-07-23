// Package oracle_networking_route_table_create creates a route table — a set of
// route rules that direct subnet traffic to gateways or other targets.
package oracle_networking_route_table_create

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
	Name         = "OCI Networking: Create Route Table"
	Description  = "Create a route table in a VCN — route rules that direct subnet traffic to a gateway or other target. Supply the rules as a JSON array, e.g. [{\"networkEntityId\":\"ocid1.internetgateway...\",\"destination\":\"0.0.0.0/0\",\"destinationType\":\"CIDR_BLOCK\"}]."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID", Placeholder: "ocid1.vcn.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "route_rules", Type: core.ConnectionTypeText, Label: "Route Rules (JSON)", Placeholder: `[{"networkEntityId":"ocid1.internetgateway.oc1..aaaa…","destination":"0.0.0.0/0","destinationType":"CIDR_BLOCK"}]`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "route_table", Type: core.ConnectionTypeObject, Label: "Route Table"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Route Table OCID"},
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
	rules, err := net.DecodeRouteRules("route_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if rules == nil {
		rules = []ocicore.RouteRule{}
	}
	details := ocicore.CreateRouteTableDetails{
		CompartmentId: &compartment,
		VcnId:         &vcnID,
		RouteRules:    rules,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateRouteTable(net.Context(), ocicore.CreateRouteTableRequest{CreateRouteTableDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	rt := net.SummariseRouteTable(&resp.RouteTable)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created route table %q (%s)", rt["display_name"], rt["lifecycle_state"]),
		"route_table":     rt,
		"id":              rt["id"],
		"lifecycle_state": rt["lifecycle_state"],
		"success":         true,
	}, nil
}
