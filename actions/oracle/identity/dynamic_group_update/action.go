// Package oracle_identity_dynamic_group_update updates a dynamic group's description, matching rule and/or freeform tags.
package oracle_identity_dynamic_group_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Update Dynamic Group"
	Description  = "Update an Oracle Cloud dynamic group's description, matching rule and/or freeform tags — only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gears"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (dynamic groups live in the root)"},
	{Name: "dynamic_group_ocid", Type: core.ConnectionTypeString, Label: "Dynamic Group OCID", Placeholder: "ocid1.dynamicgroup.oc1..aaaa… of the dynamic group to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "matching_rule", Type: core.ConnectionTypeText, Label: "Matching Rule", Placeholder: "New rule, e.g. ALL {instance.compartment.id = 'ocid1.compartment.oc1..aaaa…'} (leave blank to keep)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces all freeform tags (leave blank to keep)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dynamic_group", Type: core.ConnectionTypeObject, Label: "Dynamic Group"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Dynamic Group OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "dynamic_group_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdateDynamicGroupDetails{}
	changed := false

	if desc := iam.OptionalString("description", inputs); desc != "" {
		details.Description = &desc
		changed = true
	}
	if rule := iam.OptionalString("matching_rule", inputs); rule != "" {
		details.MatchingRule = &rule
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
		return iam.ErrorResult("nothing to update — supply a description, matching rule and/or freeform tags"), nil
	}

	resp, err := client.UpdateDynamicGroup(iam.Context(), identity.UpdateDynamicGroupRequest{DynamicGroupId: &id, UpdateDynamicGroupDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	dg := iam.SummariseDynamicGroup(&resp.DynamicGroup)
	return iam.Result(fmt.Sprintf("Updated dynamic group %q (%s)", dg["name"], dg["lifecycle_state"]), map[string]interface{}{"dynamic_group": dg, "id": dg["id"]}), nil
}
