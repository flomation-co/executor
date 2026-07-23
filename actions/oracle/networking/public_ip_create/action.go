// Package oracle_networking_public_ip_create creates a public IP — either a
// reserved address (which persists and can be reassigned) or an ephemeral one,
// optionally assigning it to a private IP.
package oracle_networking_public_ip_create

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
	Name         = "OCI Networking: Create Public IP"
	Description  = "Create a public IP — reserved (persist and reassign) or ephemeral. Optionally assign it to a private IP."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+ethernet"
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
	{Name: "lifetime", Type: core.ConnectionTypeString, Label: "Lifetime", Placeholder: "Reserved persists; ephemeral is bound to a private IP", Required: true, Options: []core.ConnectionOption{
		{Name: "Reserved", Value: "RESERVED"},
		{Name: "Ephemeral", Value: "EPHEMERAL"},
	}},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "private_ip_ocid", Type: core.ConnectionTypeString, Label: "Private IP OCID", Placeholder: "ocid1.privateip.oc1..aaaa… — assign target (required for ephemeral)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "public_ip", Type: core.ConnectionTypeObject, Label: "Public IP"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Public IP OCID"},
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
	lifetime, err := net.RequiredString("lifetime", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreatePublicIpDetails{
		CompartmentId: &compartment,
		Lifetime:      ocicore.CreatePublicIpDetailsLifetimeEnum(lifetime),
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("private_ip_ocid", inputs)); v != "" {
		details.PrivateIpId = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreatePublicIp(net.Context(), ocicore.CreatePublicIpRequest{CreatePublicIpDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	publicIP := net.SummarisePublicIp(&resp.PublicIp)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created public IP %q (%s)", publicIP["display_name"], publicIP["lifecycle_state"]),
		"public_ip":       publicIP,
		"id":              publicIP["id"],
		"lifecycle_state": publicIP["lifecycle_state"],
		"success":         true,
	}, nil
}
