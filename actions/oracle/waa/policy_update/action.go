// Package oracle_waa_policy_update applies a partial update to a Web App Acceleration policy: only
// the display name you supply is changed; a blank field is left as-is. Asynchronous — the change
// returns a work-request id, so poll Get Policy until it settles back to ACTIVE.
package oracle_waa_policy_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	wa "flomation.app/automate/executor/actions/oracle/waa"

	"github.com/oracle/oci-go-sdk/v65/waa"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Web Application Acceleration: Update Policy"
	Description  = "Partially update a Web App Acceleration policy — change only the display name you supply; a blank field is left unchanged. Asynchronous: returns a work-request id, poll Get Policy until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bolt"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Policy OCID", Placeholder: "ocid1.webappaccelerationpolicy.oc1..aaaa… — the policy to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Policy OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := wa.RequiredString("policy_ocid", inputs)
	if err != nil {
		return wa.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the display name when the operator actually supplied one.
	details := waa.UpdateWebAppAccelerationPolicyDetails{}
	if v := wa.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}

	resp, err := client.UpdateWebAppAccelerationPolicy(wa.Context(), waa.UpdateWebAppAccelerationPolicyRequest{
		WebAppAccelerationPolicyId:            &id,
		UpdateWebAppAccelerationPolicyDetails: details,
	})
	if err != nil {
		return wa.ErrorResult(auth.OCIError(err)), nil
	}
	return wa.Result(fmt.Sprintf("Updating web app acceleration policy %q — poll Get Policy until ACTIVE", id), map[string]interface{}{
		"id":              id,
		"work_request_id": wa.Str(resp.OpcWorkRequestId),
	}), nil
}
