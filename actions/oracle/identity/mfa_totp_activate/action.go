// Package oracle_identity_mfa_totp_activate activates a user's MFA TOTP device with a one-time code.
package oracle_identity_mfa_totp_activate

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Activate MFA TOTP Device"
	Description  = "Activate a user's MFA TOTP device by submitting the 6-digit code from their authenticator app — the device must be activated before it can be used for sign-in."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID (device owner)", Placeholder: "ocid1.user.oc1..aaaa… of the user the device belongs to", Required: true},
	{Name: "mfa_device_ocid", Type: core.ConnectionTypeString, Label: "MFA TOTP Device OCID", Placeholder: "ocid1.mfatotpdevice.oc1..aaaa…", Required: true},
	{Name: "totp_token", Type: core.ConnectionTypeString, Label: "TOTP Code", Placeholder: "the 6-digit code from the authenticator app", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "device", Type: core.ConnectionTypeObject, Label: "MFA TOTP Device"},
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
	token, err := iam.RequiredString("totp_token", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}

	resp, err := client.ActivateMfaTotpDevice(iam.Context(), identity.ActivateMfaTotpDeviceRequest{
		UserId:          &userID,
		MfaTotpDeviceId: &deviceID,
		MfaTotpToken:    identity.MfaTotpToken{TotpToken: &token},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	s := resp.MfaTotpDeviceSummary
	device := map[string]interface{}{
		"id":              iam.Str(s.Id),
		"user_id":         iam.Str(s.UserId),
		"lifecycle_state": string(s.LifecycleState),
		"is_activated":    s.IsActivated != nil && *s.IsActivated,
		"time_created":    iam.FormatTime(s.TimeCreated),
		"time_expires":    iam.FormatTime(s.TimeExpires),
	}
	if s.InactiveStatus != nil {
		device["inactive_status"] = *s.InactiveStatus
	}

	return iam.Result(
		fmt.Sprintf("MFA TOTP device %s is now %s (activated=%t)", iam.Str(s.Id), string(s.LifecycleState), device["is_activated"].(bool)),
		map[string]interface{}{"device": device, "id": iam.Str(s.Id)},
	), nil
}
