// Package oracle_compute_compartment_get_all lists the compartments under a
// parent compartment (or the tenancy root) — the OCI resource-grouping unit an
// operator scopes instances and other resources to.
package oracle_compute_compartment_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ociidentity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI: List Compartments"
	Description  = "List the compartments under a parent compartment (or the tenancy root) — OCI's resource-grouping unit. Optionally recurse the whole subtree."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+folder-tree"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Parent Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "subtree", Type: core.ConnectionTypeBoolean, Label: "Include whole subtree", Placeholder: "List nested compartments recursively, not just direct children"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "compartments", Type: core.ConnectionTypeObject, Label: "Compartments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	parent, err := auth.RequiredCompartment()
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.IdentityClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	subtree := compute.OptionalBool("subtree", inputs, false)
	req := ociidentity.ListCompartmentsRequest{
		CompartmentId:          compute.StringPtr(parent),
		CompartmentIdInSubtree: &subtree,
	}

	var compartments []map[string]interface{}
	for {
		resp, err := client.ListCompartments(compute.Context(), req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			c := &resp.Items[i]
			compartments = append(compartments, map[string]interface{}{
				"id":              compute.Str(c.Id),
				"name":            compute.Str(c.Name),
				"description":     compute.Str(c.Description),
				"lifecycle_state": string(c.LifecycleState),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d compartment(s)", len(compartments)),
		"compartments": compartments,
		"count":        len(compartments),
		"success":      true,
	}, nil
}
