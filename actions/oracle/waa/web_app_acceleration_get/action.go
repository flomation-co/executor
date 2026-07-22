// Package oracle_waa_web_app_acceleration_get fetches a single Web App Acceleration by OCID,
// returning its attached policy, backend load balancer and lifecycle state.
package oracle_waa_web_app_acceleration_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	wa "flomation.app/automate/executor/actions/oracle/waa"

	"github.com/oracle/oci-go-sdk/v65/waa"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Web Application Acceleration: Get Web App Acceleration"
	Description  = "Fetch a single Web App Acceleration by its OCID — its attached policy, backend load balancer and lifecycle state."
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
	{Name: "web_app_acceleration_ocid", Type: core.ConnectionTypeString, Label: "Web App Acceleration OCID", Placeholder: "ocid1.webappacceleration.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "web_app_acceleration", Type: core.ConnectionTypeObject, Label: "Web App Acceleration"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Web App Acceleration OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	accelID, err := wa.RequiredString("web_app_acceleration_ocid", inputs)
	if err != nil {
		return wa.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetWebAppAcceleration(wa.Context(), waa.GetWebAppAccelerationRequest{WebAppAccelerationId: &accelID})
	if err != nil {
		return wa.ErrorResult(auth.OCIError(err)), nil
	}
	accel := wa.SummariseWebAppAcceleration(resp.WebAppAcceleration)
	return wa.Result(fmt.Sprintf("Web App Acceleration %q (%s)", accel["display_name"], accel["lifecycle_state"]), map[string]interface{}{
		"web_app_acceleration": accel, "id": accel["id"], "lifecycle_state": accel["lifecycle_state"],
	}), nil
}
