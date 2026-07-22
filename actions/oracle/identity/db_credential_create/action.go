// Package oracle_identity_db_credential_create creates a DB credential for an IAM user.
// Unlike the other credential-create actions the operator SUPPLIES the password here (it
// is not generated), and the response carries no secret to surface.
package oracle_identity_db_credential_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create DB Credential"
	Description  = "Create a database credential for an Oracle Cloud IAM user — used to authenticate a cloud database to Identity. You supply the password here (it is not generated), so store it yourself; OCI never returns it again."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the DB credential belongs to", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this DB credential is for", Required: true},
	{Name: "db_password", Type: core.ConnectionTypeSecret, Label: "DB Password", Placeholder: "The password to set on the DB credential — store it yourself, OCI never returns it", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "db_credential", Type: core.ConnectionTypeObject, Label: "DB Credential"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DB Credential OCID"},
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
	description, err := iam.RequiredString("description", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	password, err := iam.RequiredString("db_password", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateDbCredential(iam.Context(), identity.CreateDbCredentialRequest{
		UserId: &userID,
		CreateDbCredentialDetails: identity.CreateDbCredentialDetails{
			Description: &description,
			Password:    &password,
		},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	cred := map[string]interface{}{
		"id":              iam.Str(resp.DbCredential.Id),
		"user_id":         iam.Str(resp.DbCredential.UserId),
		"lifecycle_state": string(resp.DbCredential.LifecycleState),
		"time_created":    iam.FormatTime(resp.DbCredential.TimeCreated),
		"time_expires":    iam.FormatTime(resp.DbCredential.TimeExpires),
	}
	return iam.Result(fmt.Sprintf("Created DB credential %q for the user — store the password you supplied, OCI never returns it", description), map[string]interface{}{
		"db_credential": cred, "id": cred["id"],
	}), nil
}
