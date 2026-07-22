// Package oracle_identity_api_key_delete deletes one API signing key from an IAM user by fingerprint.
package oracle_identity_api_key_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Delete API Key"
	Description  = "Remove an API signing key from an Oracle Cloud IAM user, identified by the target user's OCID and the key's fingerprint. This is permanent — the key can no longer sign requests."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID (key owner)", Placeholder: "ocid1.user.oc1..aaaa… of the user the key belongs to", Required: true},
	{Name: "fingerprint_to_delete", Type: core.ConnectionTypeString, Label: "API Key Fingerprint (to delete)", Placeholder: "aa:bb:cc:… fingerprint of the target user's API key to remove (NOT your signing key)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deleted key fingerprint"},
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
	fingerprint, err := iam.RequiredString("fingerprint_to_delete", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	_, err = client.DeleteApiKey(iam.Context(), identity.DeleteApiKeyRequest{UserId: &userID, Fingerprint: &fingerprint})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	return iam.Result(fmt.Sprintf("Deleted API key %q from user %s", fingerprint, userID), map[string]interface{}{"id": fingerprint}), nil
}
