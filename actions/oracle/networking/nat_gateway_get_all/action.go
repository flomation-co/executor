// Package oracle_networking_nat_gateway_get_all lists the NAT gateways in an
// Oracle Cloud compartment.
package oracle_networking_nat_gateway_get_all

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
	Name         = "OCI Networking: List NAT gateways"
	Description  = "List the NAT gateways in an Oracle Cloud compartment, optionally filtered by VCN."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
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
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID Filter", Placeholder: "Only NAT gateways attached to this VCN (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nat_gateways", Type: core.ConnectionTypeObject, Label: "NAT gateways"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := net.Context()

	req := ocicore.ListNatGatewaysRequest{CompartmentId: &compartment}
	if v := strings.TrimSpace(net.OptionalString("vcn_ocid", inputs)); v != "" {
		req.VcnId = &v
	}
	var items []map[string]interface{}
	truncated := false
	for page := 0; page < net.ListMaxPages; page++ {
		resp, err := client.ListNatGateways(ctx, req)
		if err != nil {
			return net.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			items = append(items, net.SummariseNatGateway(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == net.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d NAT gateway(s) in the compartment", len(items))
	if truncated {
		summary = fmt.Sprintf("Found at least %d NAT gateway(s) (list truncated at %d pages — more available)", len(items), net.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result":  summary,
		"nat_gateways": items,
		"count":        len(items),
		"truncated":    truncated,
		"success":      true,
	}, nil
}
