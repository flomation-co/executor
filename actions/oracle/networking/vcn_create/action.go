// Package oracle_networking_vcn_create creates a Virtual Cloud Network (VCN) — the
// top-level private network that holds subnets, gateways and route tables.
package oracle_networking_vcn_create

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
	Name         = "OCI Networking: Create VCN"
	Description  = "Create a Virtual Cloud Network (VCN) in an Oracle Cloud compartment — the top-level private network that holds subnets, gateways and route tables. Give it one or more CIDR blocks."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
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
	{Name: "cidr_blocks", Type: core.ConnectionTypeString, Label: "CIDR Blocks", Placeholder: "Comma-separated, e.g. 10.0.0.0/16", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "dns_label", Type: core.ConnectionTypeString, Label: "DNS Label", Placeholder: "Alphanumeric DNS label, e.g. prodvcn (optional, immutable once set)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vcn", Type: core.ConnectionTypeObject, Label: "VCN"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "VCN OCID"},
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
	cidrs := net.InputStrings("cidr_blocks", inputs)
	if len(cidrs) == 0 {
		return net.ErrorResult("at least one CIDR block is required (e.g. 10.0.0.0/16)"), nil
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreateVcnDetails{
		CompartmentId: &compartment,
		CidrBlocks:    cidrs,
		FreeformTags:  tags,
	}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("dns_label", inputs)); v != "" {
		details.DnsLabel = &v
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateVcn(net.Context(), ocicore.CreateVcnRequest{CreateVcnDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	vcn := net.SummariseVcn(&resp.Vcn)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created VCN %q (%s)", vcn["display_name"], vcn["lifecycle_state"]),
		"vcn":             vcn,
		"id":              vcn["id"],
		"lifecycle_state": vcn["lifecycle_state"],
		"success":         true,
	}, nil
}
