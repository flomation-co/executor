// Package oracle_compute_shape_get_all lists the Compute shapes (instance sizes)
// available in an OCI compartment — the catalogue an operator picks from when
// launching an instance.
package oracle_compute_shape_get_all

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
	Name         = "OCI Compute: List Shapes"
	Description  = "List the Compute shapes (instance sizes — OCPUs and memory) available in an Oracle Cloud compartment. Optionally scope to one availability domain."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "shapes", Type: core.ConnectionTypeObject, Label: "Shapes"},
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
	client, err := auth.ComputeClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	req := ocicore.ListShapesRequest{CompartmentId: compute.StringPtr(compartment)}
	if ad := strings.TrimSpace(compute.OptionalString("availability_domain", inputs)); ad != "" {
		req.AvailabilityDomain = &ad
	}

	var shapes []map[string]interface{}
	for {
		resp, err := client.ListShapes(compute.Context(), req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			s := &resp.Items[i]
			m := map[string]interface{}{"shape": compute.Str(s.Shape)}
			if s.Ocpus != nil {
				m["ocpus"] = *s.Ocpus
			}
			if s.MemoryInGBs != nil {
				m["memory_in_gbs"] = *s.MemoryInGBs
			}
			shapes = append(shapes, m)
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d shape(s)", len(shapes)),
		"shapes":      shapes,
		"count":       len(shapes),
		"success":     true,
	}, nil
}
