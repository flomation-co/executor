// Package oracle_containerengine_pod_shape_list lists the OKE pod shapes available in a compartment.
package oracle_containerengine_pod_shape_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: List Pod Shapes"
	Description  = "List the pod shapes available for virtual nodes in an Oracle Cloud OKE compartment. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
	Date         = "22/07/2026"
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "Only shapes offered in this AD, e.g. Uocm:UK-LONDON-1-AD-1 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pod_shapes", Type: core.ConnectionTypeObject, Label: "Pod shapes"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	req := okesdk.ListPodShapesRequest{CompartmentId: &compartment}
	if ad := oke.OptionalString("availability_domain", inputs); ad != "" {
		req.AvailabilityDomain = &ad
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= oke.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListPodShapes(oke.Context(), req)
		if err != nil {
			return oke.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, map[string]interface{}{"name": oke.Str(resp.Items[i].Name)})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return oke.Result(fmt.Sprintf("Found %d pod shape(s)", len(out)), map[string]interface{}{
		"pod_shapes": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
