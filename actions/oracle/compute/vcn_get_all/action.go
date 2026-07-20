// Package oracle_compute_vcn_get_all lists the Virtual Cloud Networks (VCNs) in
// an OCI compartment — the top-level networks instances and subnets live in.
package oracle_compute_vcn_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: List VCNs"
	Description  = "List the Virtual Cloud Networks (VCNs) in an Oracle Cloud compartment — the networks that hold subnets and instances."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
	Date         = "20/07/2026"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name filter", Placeholder: "Only VCNs with this exact name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vcns", Type: core.ConnectionTypeObject, Label: "VCNs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	req := ocicore.ListVcnsRequest{CompartmentId: compute.StringPtr(compartment)}
	if dn := strings.TrimSpace(compute.OptionalString("display_name", inputs)); dn != "" {
		req.DisplayName = &dn
	}

	var vcns []map[string]interface{}
	for {
		resp, err := client.ListVcns(compute.Context(), req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			v := &resp.Items[i]
			vcns = append(vcns, map[string]interface{}{
				"id":              compute.Str(v.Id),
				"display_name":    compute.Str(v.DisplayName),
				"cidr_block":      compute.Str(v.CidrBlock),
				"lifecycle_state": string(v.LifecycleState),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d VCN(s)", len(vcns)),
		"vcns":        vcns,
		"count":       len(vcns),
		"success":     true,
	}, nil
}
