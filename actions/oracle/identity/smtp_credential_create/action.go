// Package oracle_identity_smtp_credential_create creates an SMTP credential for a user.
// The Oracle-generated password is returned ONCE, on create, and never again.
package oracle_identity_smtp_credential_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create SMTP Credential"
	Description  = "Create an SMTP credential for an Oracle Cloud IAM user — an Oracle-generated username/password pair for sending mail through Email Delivery. The password is returned ONCE here and never shown again, so capture it now."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the user picker)"},
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the SMTP credential belongs to", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this SMTP credential is for", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "smtp_credential", Type: core.ConnectionTypeObject, Label: "SMTP Credential"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "SMTP Username"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "SMTP Password (shown once)"},
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
	description, err := iam.RequiredString("description", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateSmtpCredential(iam.Context(), identity.CreateSmtpCredentialRequest{
		UserId:                      &userID,
		CreateSmtpCredentialDetails: identity.CreateSmtpCredentialDetails{Description: &description},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	cred := &resp.SmtpCredential
	// No shared summariser for SMTP credentials — build the map inline, deliberately
	// omitting the one-time password from the object (surfaced as its own output).
	credential := map[string]interface{}{
		"id":              iam.Str(cred.Id),
		"user_id":         iam.Str(cred.UserId),
		"username":        iam.Str(cred.Username),
		"description":     iam.Str(cred.Description),
		"lifecycle_state": string(cred.LifecycleState),
		"time_created":    iam.FormatTime(cred.TimeCreated),
		"time_expires":    iam.FormatTime(cred.TimeExpires),
	}
	return iam.Result(fmt.Sprintf("Created SMTP credential %q — capture the password now, it is shown only once", description), map[string]interface{}{
		"smtp_credential": credential,
		"username":        iam.Str(cred.Username),
		"password":        iam.Str(cred.Password),
		"id":              credential["id"],
	}), nil
}
