// Package oracle_identity_membership_get reads one IAM user-group membership by OCID.
package oracle_identity_membership_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Group Membership"
	Description  = "Fetch a single Oracle Cloud IAM user-group membership by OCID — the user, group and compartment it ties together, plus its lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+user-plus"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the pickers)"},
	{Name: "membership_ocid", Type: core.ConnectionTypeString, Label: "Membership OCID", Placeholder: "ocid1.usergroupmembership.oc1..aaaa… of the membership to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "membership", Type: core.ConnectionTypeObject, Label: "Membership"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Membership OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "membership_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetUserGroupMembership(iam.Context(), identity.GetUserGroupMembershipRequest{UserGroupMembershipId: &id})
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
	return iam.Result(fmt.Sprintf("Membership %s is %s", iam.Str(m.Id), membership["lifecycle_state"]), map[string]interface{}{"membership": membership, "id": membership["id"]}), nil
}
