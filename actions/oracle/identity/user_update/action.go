// Package oracle_identity_user_update updates the description, email and freeform tags of an IAM user.
package oracle_identity_user_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Update User"
	Description  = "Update an Oracle Cloud IAM user's description, email and/or freeform tags — only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+user"
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID (to update)", Placeholder: "ocid1.user.oc1..aaaa… of the user to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "New email — must be unique across the tenancy (leave blank to keep)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"Department":"Finance"} — replaces all freeform tags (leave blank to keep)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "User OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "target_user_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdateUserDetails{}
	changed := false

	if desc := iam.OptionalString("description", inputs); desc != "" {
		details.Description = &desc
		changed = true
	}
	if email := iam.OptionalString("email", inputs); email != "" {
		details.Email = &email
		changed = true
	}
	tags, err := iam.FreeformTags("tags", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	if tags != nil {
		details.FreeformTags = tags
		changed = true
	}

	if !changed {
		return iam.ErrorResult("nothing to update — supply a description, email and/or freeform tags"), nil
	}

	resp, err := client.UpdateUser(iam.Context(), identity.UpdateUserRequest{UserId: &id, UpdateUserDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	user := iam.SummariseUser(&resp.User)
	return iam.Result(fmt.Sprintf("Updated user %q (%s)", user["name"], user["lifecycle_state"]), map[string]interface{}{"user": user, "id": user["id"]}), nil
}
