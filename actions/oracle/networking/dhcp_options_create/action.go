// Package oracle_networking_dhcp_options_create creates a set of DHCP options in a
// VCN — the DNS resolution configuration that subnets attach to.
package oracle_networking_dhcp_options_create

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
	Name         = "OCI Networking: Create DHCP Options"
	Description  = "Create a DHCP options set in a VCN — the DNS resolution config subnets use. Supply the options as a JSON array."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gear"
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
	{Name: "options_json", Type: core.ConnectionTypeText, Label: "DHCP Options (JSON)", Placeholder: `[{"type":"DomainNameServer","serverType":"VcnLocalPlusInternet"},{"type":"SearchDomain","searchDomainNames":["example.com"]}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dhcp_options", Type: core.ConnectionTypeObject, Label: "DHCP Options"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DHCP Options OCID"},
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
	options, err := net.DecodeDhcpOptions("options_json", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if len(options) == 0 {
		return net.ErrorResult(`at least one DHCP option is required (a JSON array of "DomainNameServer" and/or "SearchDomain" objects)`), nil
	}
	details := ocicore.CreateDhcpDetails{
		CompartmentId: &compartment,
		VcnId:         &vcnID,
		Options:       options,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateDhcpOptions(net.Context(), ocicore.CreateDhcpOptionsRequest{CreateDhcpDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	d := net.SummariseDhcpOptions(&resp.DhcpOptions)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created DHCP options %q (%s)", d["display_name"], d["lifecycle_state"]),
		"dhcp_options":    d,
		"id":              d["id"],
		"lifecycle_state": d["lifecycle_state"],
		"success":         true,
	}, nil
}
