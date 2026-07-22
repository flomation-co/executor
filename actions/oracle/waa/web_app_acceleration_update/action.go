// Package oracle_waa_web_app_acceleration_update applies a partial update to a Web App
// Acceleration: only the display name and/or the attached acceleration policy you supply are
// changed; blank fields are left as-is. Asynchronous — the change returns a work-request id, so
// poll Get Web App Acceleration until it settles back to ACTIVE.
package oracle_waa_web_app_acceleration_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	wa "flomation.app/automate/executor/actions/oracle/waa"

	"github.com/oracle/oci-go-sdk/v65/waa"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Web Application Acceleration: Update Web App Acceleration"
	Description  = "Partially update a Web App Acceleration — change only the display name and/or the attached acceleration policy you supply; blank fields are left unchanged. Asynchronous: returns a work-request id, poll Get Web App Acceleration until ACTIVE."
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
	{Name: "web_app_acceleration_ocid", Type: core.ConnectionTypeString, Label: "Web App Acceleration OCID", Placeholder: "ocid1.webappacceleration.oc1..aaaa… — the one to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Acceleration Policy OCID", Placeholder: "ocid1.webappaccelerationpolicy.oc1..aaaa… to attach (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Web App Acceleration OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := wa.RequiredString("web_app_acceleration_ocid", inputs)
	if err != nil {
		return wa.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied.
	details := waa.UpdateWebAppAccelerationDetails{}
	if v := wa.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := wa.OptionalString("policy_ocid", inputs); v != "" {
		details.WebAppAccelerationPolicyId = &v
	}

	resp, err := client.UpdateWebAppAcceleration(wa.Context(), waa.UpdateWebAppAccelerationRequest{
		WebAppAccelerationId:            &id,
		UpdateWebAppAccelerationDetails: details,
	})
	if err != nil {
		return wa.ErrorResult(auth.OCIError(err)), nil
	}
	return wa.Result(fmt.Sprintf("Updating web app acceleration %q — poll Get Web App Acceleration until ACTIVE", id), map[string]interface{}{
		"id":              id,
		"work_request_id": wa.Str(resp.OpcWorkRequestId),
	}), nil
}
