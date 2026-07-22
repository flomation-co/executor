// Package oracle_identity_user_create_or_reset_ui_password creates or resets a user's
// Console (UI) sign-in password. The generated password is returned ONCE, on this call,
// and never shown again.
package oracle_identity_user_create_or_reset_ui_password

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create or Reset UI Password"
	Description  = "Create or reset an Oracle Cloud IAM user's Console (UI) sign-in password. A new one-time password is generated and returned ONCE here — it is never shown again, so capture it now. If the user already has a password, this resets it."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… whose UI password to create/reset", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "UI Password (shown once)"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "target_user_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.CreateOrResetUIPassword(iam.Context(), identity.CreateOrResetUIPasswordRequest{UserId: &id})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	userID := iam.Str(resp.UiPassword.UserId)
	if userID == "" {
		userID = id
	}
	return iam.Result(
		fmt.Sprintf("Created/reset the Console password for user %q (%s) — capture the password value now, it is shown only once", userID, string(resp.UiPassword.LifecycleState)),
		map[string]interface{}{
			"password": iam.Str(resp.UiPassword.Password),
			"user_id":  userID,
		},
	), nil
}
