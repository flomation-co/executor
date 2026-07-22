// Package oracle_identity_swift_password_create creates a Swift password for a user. The
// password value is Oracle-generated and returned ONCE, on create, and never again.
package oracle_identity_swift_password_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create Swift Password"
	Description  = "Create a Swift password for an Oracle Cloud IAM user — an Oracle-generated credential for Swift-client access to Object Storage. The password value is returned ONCE here and never shown again, so capture it now. (Swift passwords are deprecated in favour of auth tokens.)"
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the Swift password belongs to", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this Swift password is for", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "swift_password", Type: core.ConnectionTypeObject, Label: "Swift Password"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "Password (shown once)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Swift Password OCID"},
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
	resp, err := client.CreateSwiftPassword(iam.Context(), identity.CreateSwiftPasswordRequest{
		UserId:                     &userID,
		CreateSwiftPasswordDetails: identity.CreateSwiftPasswordDetails{Description: &description},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	sp := resp.SwiftPassword
	summary := map[string]interface{}{
		"id":              iam.Str(sp.Id),
		"user_id":         iam.Str(sp.UserId),
		"description":     iam.Str(sp.Description),
		"lifecycle_state": string(sp.LifecycleState),
		"time_created":    iam.FormatTime(sp.TimeCreated),
		"expires_on":      iam.FormatTime(sp.ExpiresOn),
	}
	return iam.Result(fmt.Sprintf("Created Swift password %q — capture the password value now, it is shown only once", description), map[string]interface{}{
		"swift_password": summary, "password": iam.Str(sp.Password), "id": summary["id"],
	}), nil
}
