// Package oracle_compute_subnet_get_all lists the subnets in an OCI compartment,
// optionally within one VCN — the network an instance's VNIC is placed in at
// launch.
package oracle_compute_subnet_get_all

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
	Name         = "OCI Compute: List Subnets"
	Description  = "List the subnets in an Oracle Cloud compartment, optionally within one VCN — the network an instance's VNIC is placed in at launch."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID", Placeholder: "Only subnets in this VCN (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subnets", Type: core.ConnectionTypeObject, Label: "Subnets"},
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
	req := ocicore.ListSubnetsRequest{CompartmentId: compute.StringPtr(compartment)}
	if vcn := strings.TrimSpace(compute.OptionalString("vcn_ocid", inputs)); vcn != "" {
		req.VcnId = &vcn
	}

	var subnets []map[string]interface{}
	for {
		resp, err := client.ListSubnets(compute.Context(), req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			s := &resp.Items[i]
			m := map[string]interface{}{
				"id":                  compute.Str(s.Id),
				"display_name":        compute.Str(s.DisplayName),
				"cidr_block":          compute.Str(s.CidrBlock),
				"vcn_id":              compute.Str(s.VcnId),
				"availability_domain": compute.Str(s.AvailabilityDomain),
				"lifecycle_state":     string(s.LifecycleState),
			}
			// Whether a VNIC in this subnet can get a public IP — the operator needs
			// this to pick a subnet for Launch Instance + Assign Public IP.
			if s.ProhibitPublicIpOnVnic != nil {
				m["public_ip_allowed"] = !*s.ProhibitPublicIpOnVnic
			}
			subnets = append(subnets, m)
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d subnet(s)", len(subnets)),
		"subnets":     subnets,
		"count":       len(subnets),
		"success":     true,
	}, nil
}
