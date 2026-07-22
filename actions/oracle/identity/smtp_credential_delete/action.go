// Package oracle_identity_smtp_credential_delete permanently deletes an IAM user's SMTP credential.
package oracle_identity_smtp_credential_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Delete SMTP Credential"
	Description  = "Permanently delete an SMTP credential belonging to an Oracle Cloud IAM user, revoking its ability to send mail through the Email Delivery service. Synchronous."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the SMTP credential belongs to", Required: true},
	{Name: "smtp_credential_ocid", Type: core.ConnectionTypeString, Label: "SMTP Credential OCID", Placeholder: "ocid1.credential.oc1..aaaa… of the SMTP credential to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "SMTP Credential OCID"},
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
	credentialID, err := iam.RequiredString("smtp_credential_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	if _, err := client.DeleteSmtpCredential(iam.Context(), identity.DeleteSmtpCredentialRequest{
		UserId:           &userID,
		SmtpCredentialId: &credentialID,
	}); err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	return iam.Result(fmt.Sprintf("Deleted SMTP credential %s", credentialID), map[string]interface{}{"id": credentialID}), nil
}
