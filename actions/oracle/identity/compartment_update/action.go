// Package oracle_identity_compartment_update updates the name, description and freeform tags of a compartment.
package oracle_identity_compartment_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Update Compartment"
	Description  = "Update an Oracle Cloud compartment's name, description and/or freeform tags — only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+folder-tree"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the compartment picker)"},
	{Name: "target_compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID (to update)", Placeholder: "ocid1.compartment.oc1..aaaa… of the compartment to update", Required: true},
	{Name: "compartment_name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "New name — must be unique within the parent (leave blank to keep)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces all freeform tags (leave blank to keep)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "compartment", Type: core.ConnectionTypeObject, Label: "Compartment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "target_compartment_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdateCompartmentDetails{}
	changed := false

	if name := iam.OptionalString("compartment_name", inputs); name != "" {
		details.Name = &name
		changed = true
	}
	if desc := iam.OptionalString("description", inputs); desc != "" {
		details.Description = &desc
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
		return iam.ErrorResult("nothing to update — supply a name, description and/or freeform tags"), nil
	}

	resp, err := client.UpdateCompartment(iam.Context(), identity.UpdateCompartmentRequest{CompartmentId: &id, UpdateCompartmentDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	compartment := iam.SummariseCompartment(&resp.Compartment)
	return iam.Result(fmt.Sprintf("Updated compartment %q (%s)", compartment["name"], compartment["lifecycle_state"]), map[string]interface{}{"compartment": compartment, "id": compartment["id"]}), nil
}
