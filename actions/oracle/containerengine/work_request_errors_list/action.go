// Package oracle_containerengine_work_request_errors_list lists the errors recorded
// against a single OKE work request.
package oracle_containerengine_work_request_errors_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: List Work Request Errors"
	Description  = "List the errors recorded against an Oracle Cloud OKE work request."
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
	{Name: "work_request_ocid", Type: core.ConnectionTypeString, Label: "Work Request OCID", Placeholder: "ocid1.clustersworkrequest.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "errors", Type: core.ConnectionTypeObject, Label: "Errors"},
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
	wrid, err := oke.RequiredString("work_request_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	resp, err := client.ListWorkRequestErrors(oke.Context(), okesdk.ListWorkRequestErrorsRequest{
		CompartmentId: &compartment,
		WorkRequestId: &wrid,
	})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	out := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		e := resp.Items[i]
		out = append(out, map[string]interface{}{
			"code":      oke.Str(e.Code),
			"message":   oke.Str(e.Message),
			"timestamp": oke.FormatTime(e.Timestamp),
		})
	}
	return oke.Result(fmt.Sprintf("Found %d work request error(s)", len(out)), map[string]interface{}{
		"errors": out, "count": fmt.Sprintf("%d", len(out)), "truncated": false,
	}), nil
}
