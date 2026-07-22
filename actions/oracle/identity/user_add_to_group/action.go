// Package oracle_identity_user_add_to_group adds a user to a group, creating the
// user-group membership that grants the user the group's policies.
package oracle_identity_user_add_to_group

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Add User to Group"
	Description  = "Add an Oracle Cloud IAM user to a group — creating the membership that grants the user every policy the group is named in. Returns the membership OCID (use it to remove the user later)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+user-plus"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the user/group pickers)"},
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… of the user to add", Required: true},
	{Name: "group_ocid", Type: core.ConnectionTypeString, Label: "Group OCID", Placeholder: "ocid1.group.oc1..aaaa… of the group to add them to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "membership", Type: core.ConnectionTypeObject, Label: "Membership"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Membership OCID"},
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
	groupID, err := iam.RequiredString("group_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	resp, err := client.AddUserToGroup(iam.Context(), identity.AddUserToGroupRequest{
		AddUserToGroupDetails: identity.AddUserToGroupDetails{UserId: &userID, GroupId: &groupID},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	m := resp.UserGroupMembership
	membership := map[string]interface{}{
		"id":              iam.Str(m.Id),
		"user_id":         iam.Str(m.UserId),
		"group_id":        iam.Str(m.GroupId),
		"compartment_id":  iam.Str(m.CompartmentId),
		"lifecycle_state": string(m.LifecycleState),
		"time_created":    iam.FormatTime(m.TimeCreated),
	}
	return iam.Result(fmt.Sprintf("Added user to group (membership %s)", iam.Str(m.Id)), map[string]interface{}{"membership": membership, "id": membership["id"]}), nil
}
