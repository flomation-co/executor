// Package oracle_identity_group_update updates an IAM group's description and freeform tags.
package oracle_identity_group_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Update Group"
	Description  = "Update an Oracle Cloud IAM group's description and/or freeform tags. Only the fields you supply are changed; the group name is immutable."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+user-group"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (IAM groups live in the root)"},
	{Name: "group_ocid", Type: core.ConnectionTypeString, Label: "Group OCID", Placeholder: "ocid1.group.oc1..aaaa… of the group to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"team":"ops"} — replaces all freeform tags (leave blank to keep current)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "group", Type: core.ConnectionTypeObject, Label: "Group"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "group_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdateGroupDetails{}
	if iam.BoolWasSet("description", inputs) {
		desc := strings.TrimSpace(iam.OptionalString("description", inputs))
		details.Description = &desc
	}
	tags, err := iam.FreeformTags("tags", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateGroup(iam.Context(), identity.UpdateGroupRequest{GroupId: &id, UpdateGroupDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	group := iam.SummariseGroup(&resp.Group)
	return iam.Result(fmt.Sprintf("Updated group %q", group["name"]), map[string]interface{}{"group": group, "id": group["id"]}), nil
}
