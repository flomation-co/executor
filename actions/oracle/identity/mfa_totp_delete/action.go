// Package oracle_identity_mfa_totp_delete deletes an IAM user's MFA TOTP device by OCID.
package oracle_identity_mfa_totp_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Delete MFA TOTP Device"
	Description  = "Delete an MFA TOTP device from an Oracle Cloud IAM user — removes that authenticator so the user must re-enrol to use MFA again. Give the user's OCID and the MFA TOTP device's OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the user picker)"},
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the device belongs to", Required: true},
	{Name: "mfa_device_ocid", Type: core.ConnectionTypeString, Label: "MFA TOTP Device OCID", Placeholder: "ocid1.mfatotpdevice.oc1..aaaa… of the device to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "MFA TOTP Device OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	userID, err := iam.RequiredString("target_user_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	deviceID, err := iam.RequiredString("mfa_device_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	if _, err := client.DeleteMfaTotpDevice(iam.Context(), identity.DeleteMfaTotpDeviceRequest{
		UserId:          &userID,
		MfaTotpDeviceId: &deviceID,
	}); err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	return iam.Result(fmt.Sprintf("Deleted MFA TOTP device %q from user %q", deviceID, userID), map[string]interface{}{"id": deviceID}), nil
}
