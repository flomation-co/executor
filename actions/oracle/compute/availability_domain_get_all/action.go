// Package oracle_compute_availability_domain_get_all lists the availability
// domains in the tenancy's region — the physical fault-isolation zones an
// instance is launched into.
package oracle_compute_availability_domain_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ociidentity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: List Availability Domains"
	Description  = "List the availability domains in the region — the fault-isolation zones an instance is launched into. Needed to pick a domain for List Shapes / Launch Instance."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+layer-group"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "availability_domains", Type: core.ConnectionTypeObject, Label: "Availability Domains"},
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
	client, err := auth.IdentityClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.ListAvailabilityDomains(compute.Context(), ociidentity.ListAvailabilityDomainsRequest{
		CompartmentId: compute.StringPtr(compartment),
	})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	var ads []map[string]interface{}
	for i := range resp.Items {
		ad := &resp.Items[i]
		ads = append(ads, map[string]interface{}{
			"name": compute.Str(ad.Name),
			"id":   compute.Str(ad.Id),
		})
	}
	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Found %d availability domain(s)", len(ads)),
		"availability_domains": ads,
		"count":                len(ads),
		"success":              true,
	}, nil
}
