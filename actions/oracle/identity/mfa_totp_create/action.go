// Package oracle_identity_mfa_totp_create registers an MFA TOTP device for an IAM user.
package oracle_identity_mfa_totp_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create MFA TOTP Device"
	Description  = "Register a multi-factor-authentication TOTP device for an Oracle Cloud IAM user. This starts enrolment: the device comes back in the CREATING state — you must then generate its seed and activate it (with a TOTP code) to finish, and MFA registration ultimately requires the Console."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the MFA device belongs to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mfa_totp_device", Type: core.ConnectionTypeObject, Label: "MFA TOTP Device"},
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
	resp, err := client.CreateMfaTotpDevice(iam.Context(), identity.CreateMfaTotpDeviceRequest{UserId: &userID})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	d := resp.MfaTotpDevice
	device := map[string]interface{}{
		"id":              iam.Str(d.Id),
		"user_id":         iam.Str(d.UserId),
		"lifecycle_state": string(d.LifecycleState),
		"is_activated":    d.IsActivated != nil && *d.IsActivated,
		"time_created":    iam.FormatTime(d.TimeCreated),
		"time_expires":    iam.FormatTime(d.TimeExpires),
	}
	if d.InactiveStatus != nil {
		device["inactive_status"] = *d.InactiveStatus
	}
	// Seed is normally empty on create (you call Generate TOTP Seed next) — surface it if present.
	if s := iam.Str(d.Seed); s != "" {
		device["seed"] = s
	}
	return iam.Result(
		fmt.Sprintf("Registered MFA TOTP device %s (%s) — now generate its seed and activate it to finish enrolment", iam.Str(d.Id), string(d.LifecycleState)),
		map[string]interface{}{"mfa_totp_device": device, "id": iam.Str(d.Id)},
	), nil
}
