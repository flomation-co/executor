// Package oracle_networking_security_list_create creates a security list — a set of
// stateful/stateless ingress and egress firewall rules that attach to subnets.
package oracle_networking_security_list_create

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
	Name         = "OCI Networking: Create Security List"
	Description  = "Create a security list in a VCN — a set of ingress and egress firewall rules that subnets attach to. Supply the rules as JSON arrays, e.g. ingress [{\"protocol\":\"6\",\"source\":\"0.0.0.0/0\",\"tcpOptions\":{\"destinationPortRange\":{\"min\":443,\"max\":443}}}]."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "ingress_security_rules", Type: core.ConnectionTypeText, Label: "Ingress Rules (JSON)", Placeholder: `[{"protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":443,"max":443}}}]`},
	{Name: "egress_security_rules", Type: core.ConnectionTypeText, Label: "Egress Rules (JSON)", Placeholder: `[{"protocol":"all","destination":"0.0.0.0/0"}]`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_list", Type: core.ConnectionTypeObject, Label: "Security List"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Security List OCID"},
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
	ingress, err := net.DecodeIngressRules("ingress_security_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	egress, err := net.DecodeEgressRules("egress_security_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	// The SDK requires both arrays present; an empty list (no rules) is valid.
	if ingress == nil {
		ingress = []ocicore.IngressSecurityRule{}
	}
	if egress == nil {
		egress = []ocicore.EgressSecurityRule{}
	}
	details := ocicore.CreateSecurityListDetails{
		CompartmentId:        &compartment,
		VcnId:                &vcnID,
		IngressSecurityRules: ingress,
		EgressSecurityRules:  egress,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateSecurityList(net.Context(), ocicore.CreateSecurityListRequest{CreateSecurityListDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	sl := net.SummariseSecurityList(&resp.SecurityList)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created security list %q (%s)", sl["display_name"], sl["lifecycle_state"]),
		"security_list":   sl,
		"id":              sl["id"],
		"lifecycle_state": sl["lifecycle_state"],
		"success":         true,
	}, nil
}
